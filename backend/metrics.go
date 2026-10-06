package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os/exec"
	"os/user"
	"strings"
	"time"

	gnet "github.com/shirou/gopsutil/v4/net"
)

type networkSample struct {
	Received, Sent uint64
	At             time.Time
	RX, TX         float64
}

func (a *App) sampleNetwork() {
	counters, err := gnet.IOCounters(true)
	if err != nil {
		return
	}
	now := time.Now()
	a.metrics.Lock()
	defer a.metrics.Unlock()
	if a.networkLast == nil {
		a.networkLast = map[string]networkSample{}
	}
	next := map[string]networkSample{}
	for _, c := range counters {
		if c.Name == "lo" || c.Name == "lo0" {
			continue
		}
		s := networkSample{Received: c.BytesRecv, Sent: c.BytesSent, At: now}
		old, ok := a.networkLast[c.Name]
		if ok {
			elapsed := now.Sub(old.At).Seconds()
			if elapsed > 0 && c.BytesRecv >= old.Received && c.BytesSent >= old.Sent {
				s.RX = float64(c.BytesRecv-old.Received) / elapsed
				s.TX = float64(c.BytesSent-old.Sent) / elapsed
			}
		}
		next[c.Name] = s
	}
	a.networkLast = next
}
func (a *App) monitorNetwork() {
	a.sampleNetwork()
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			a.sampleNetwork()
		}
	}()
}
func (a *App) network(w http.ResponseWriter, r *http.Request) {
	a.metrics.Lock()
	defer a.metrics.Unlock()
	out := []map[string]any{}
	var rx, tx float64
	for name, s := range a.networkLast {
		rx += s.RX
		tx += s.TX
		out = append(out, map[string]any{"name": name, "rxBytesPerSecond": s.RX, "txBytesPerSecond": s.TX, "received": s.Received, "sent": s.Sent, "sampledAt": s.At})
	}
	jsonOut(w, map[string]any{"interfaces": out, "rxBytesPerSecond": rx, "txBytesPerSecond": tx, "available": len(out) > 0})
}
func smartCommand(ctx context.Context, args ...string) ([]byte, error) {
	binary, err := exec.LookPath("smartctl")
	if err != nil {
		return nil, err
	}
	if env("SMART_USE_SUDO", "false") == "true" {
		return exec.CommandContext(ctx, "sudo", append([]string{"-n", binary}, args...)...).CombinedOutput()
	}
	return exec.CommandContext(ctx, binary, args...).CombinedOutput()
}
func (a *App) hardware(w http.ResponseWriter, r *http.Request) {
	a.metrics.Lock()
	cached, at := a.smartCached, a.smartAt
	a.metrics.Unlock()
	if cached != nil && time.Since(at) < time.Minute && r.URL.Query().Get("refresh") != "true" {
		jsonOut(w, cached)
		return
	}
	result := map[string]any{"available": false, "disks": []any{}, "message": "SMART data unavailable. Install smartmontools and grant the API service access to the intended disks.", "sampledAt": time.Now()}
	if current, err := user.Current(); err == nil {
		result["serviceUser"] = current.Username
	}
	if _, err := exec.LookPath("smartctl"); err == nil {
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		raw, err := smartCommand(ctx, "--scan-open", "-j")
		var scan struct{ Devices []struct{ Name, Type string } }
		if json.Unmarshal(raw, &scan) == nil && len(scan.Devices) > 0 {
			disks := []map[string]any{}
			for i, device := range scan.Devices {
				if i >= 16 {
					break
				}
				args := []string{"-a", "-j", device.Name}
				if device.Type != "" {
					args = append(args, "-d", device.Type)
				}
				deviceCtx, stop := context.WithTimeout(ctx, 3*time.Second)
				data, cmdErr := smartCommand(deviceCtx, args...)
				timedOut := deviceCtx.Err() != nil
				stop()
				var info struct {
					Smartctl struct {
						Messages   []struct{ String, Severity string }
						ExitStatus int `json:"exit_status"`
					} `json:"smartctl"`
					Product     string                 `json:"product"`
					Vendor      string                 `json:"vendor"`
					ModelName   string                 `json:"model_name"`
					Serial      string                 `json:"serial_number"`
					SmartStatus *struct{ Passed bool } `json:"smart_status"`
					Temperature *struct{ Current int } `json:"temperature"`
					PowerOn     *struct{ Hours int64 } `json:"power_on_time"`
					NVMe        *struct {
						Temperature     int
						CriticalWarning int `json:"critical_warning"`
						PercentageUsed  int `json:"percentage_used"`
					} `json:"nvme_smart_health_information_log"`
				}
				disk := map[string]any{"device": device.Name, "type": device.Type, "health": "unavailable", "temperature": nil, "model": "Unknown", "powerOnHours": nil}
				if json.Unmarshal(data, &info) == nil {
					if info.ModelName != "" {
						disk["model"] = info.ModelName
					} else if info.Product != "" {
						disk["model"] = strings.TrimSpace(info.Vendor + " " + info.Product)
					}
					messages := []string{}
					for _, m := range info.Smartctl.Messages {
						if m.Severity == "error" || m.Severity == "warning" {
							messages = append(messages, m.String)
						}
					}
					if len(messages) > 0 {
						disk["message"] = strings.Join(messages, "; ")
						if strings.Contains(strings.ToLower(strings.Join(messages, " ")), "permission denied") {
							disk["accessDenied"] = true
						}
					}
					disk["exitStatus"] = info.Smartctl.ExitStatus
					if info.SmartStatus != nil {
						if info.SmartStatus.Passed {
							disk["health"] = "passed"
						} else {
							disk["health"] = "failed"
						}
					}
					if info.Temperature != nil {
						disk["temperature"] = info.Temperature.Current
					} else if info.NVMe != nil && info.NVMe.Temperature > 0 {
						disk["temperature"] = info.NVMe.Temperature
					}
					if info.PowerOn != nil {
						disk["powerOnHours"] = info.PowerOn.Hours
					}
					if info.NVMe != nil {
						disk["wearPercent"] = info.NVMe.PercentageUsed
						if info.NVMe.CriticalWarning != 0 {
							disk["health"] = "failed"
						}
					}
				}
				if timedOut {
					disk["message"] = "SMART query timed out. Try refreshing the readings."
				} else if cmdErr != nil && disk["message"] == nil && disk["health"] == "unavailable" {
					detail := strings.TrimSpace(string(data))
					if len(detail) > 500 {
						detail = detail[:500]
					}
					if detail == "" {
						detail = cmdErr.Error()
					}
					disk["message"] = detail
					if strings.Contains(strings.ToLower(detail), "authentication") || strings.Contains(strings.ToLower(detail), "password") {
						disk["accessDenied"] = true
					}
				}
				disks = append(disks, disk)
			}
			result["disks"] = disks
			readable := 0
			denied := 0
			for _, d := range disks {
				if d["health"] != "unavailable" || d["temperature"] != nil {
					readable++
				}
				if d["accessDenied"] == true {
					denied++
				}
			}
			result["available"] = readable > 0
			result["accessDenied"] = denied > 0
			if denied > 0 {
				result["message"] = "The API account cannot read one or more disks. Enable the scoped SMART access described below."
			} else {
				result["message"] = "SMART readings refresh every minute; unsupported fields are unavailable."
			}
		} else if err != nil {
			detail := strings.TrimSpace(string(raw))
			if len(detail) > 500 {
				detail = detail[:500]
			}
			if detail == "" {
				detail = err.Error()
			}
			result["message"] = "SMART scan failed: " + detail
			if strings.Contains(strings.ToLower(detail), "authentication") || strings.Contains(strings.ToLower(detail), "password") || strings.Contains(strings.ToLower(detail), "permission denied") {
				result["accessDenied"] = true
			}
		} else {
			result["message"] = "No SMART-capable disks were detected."
		}
	}
	a.metrics.Lock()
	a.smartCached = result
	a.smartAt = time.Now()
	a.metrics.Unlock()
	jsonOut(w, result)
}
