package metrics

import (
	"net/http"
	"time"

	"github.com/rajibdpi/trynas/core"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	gnet "github.com/shirou/gopsutil/v4/net"
)

// systemSnapshot shares one sample across dashboard clients for one second.
// Holding this dedicated lock also prevents concurrent cache misses from
// starting duplicate CPU measurements; the network lock stays independent.
func (a *Module) systemSnapshot() (map[string]any, error) {
	a.systemMu.Lock()
	defer a.systemMu.Unlock()
	if a.systemCached != nil && time.Since(a.systemAt) < time.Second {
		return a.systemCached, nil
	}
	v, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	c, err := cpu.Percent(200*time.Millisecond, false)
	if err != nil {
		return nil, err
	}
	h, err := host.Info()
	if err != nil {
		return nil, err
	}
	du, err := disk.Usage(a.Root)
	if err != nil {
		return nil, err
	}
	pct := 0.0
	if len(c) > 0 {
		pct = c[0]
	}
	result := map[string]any{"cpu": pct, "memoryPercent": v.UsedPercent, "memoryUsed": v.Used, "memoryTotal": v.Total, "uptime": h.Uptime, "hostname": h.Hostname, "os": h.Platform, "kernel": h.KernelVersion, "storage": map[string]any{"total": du.Total, "used": du.Used, "free": du.Free, "percent": du.UsedPercent, "path": a.Root}}
	a.systemCached, a.systemAt = result, time.Now()
	return result, nil
}

func (a *Module) System(w http.ResponseWriter, r *http.Request) {
	result, err := a.systemSnapshot()
	if err != nil {
		core.Bad(w, "system metrics unavailable", http.StatusServiceUnavailable)
		return
	}
	core.JSON(w, result)
}

func (a *Module) SampleNetwork() {
	counters, err := gnet.IOCounters(true)
	if err != nil {
		return
	}
	now := time.Now()
	a.Metrics.Lock()
	defer a.Metrics.Unlock()
	if a.NetworkLast == nil {
		a.NetworkLast = map[string]core.NetworkSample{}
	}
	next := map[string]core.NetworkSample{}
	for _, c := range counters {
		if c.Name == "lo" || c.Name == "lo0" {
			continue
		}
		s := core.NetworkSample{Received: c.BytesRecv, Sent: c.BytesSent, At: now}
		old, ok := a.NetworkLast[c.Name]
		if ok {
			elapsed := now.Sub(old.At).Seconds()
			if elapsed > 0 && c.BytesRecv >= old.Received && c.BytesSent >= old.Sent {
				s.RX = float64(c.BytesRecv-old.Received) / elapsed
				s.TX = float64(c.BytesSent-old.Sent) / elapsed
			}
		}
		next[c.Name] = s
	}
	a.NetworkLast = next
}

func (a *Module) MonitorNetwork() {
	a.SampleNetwork()
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for range ticker.C {
			a.SampleNetwork()
		}
	}()
}

func (a *Module) Network(w http.ResponseWriter, r *http.Request) {
	a.Metrics.Lock()
	defer a.Metrics.Unlock()
	out := []map[string]any{}
	var rx, tx float64
	for name, s := range a.NetworkLast {
		rx += s.RX
		tx += s.TX
		out = append(out, map[string]any{"name": name, "rxBytesPerSecond": s.RX, "txBytesPerSecond": s.TX, "received": s.Received, "sent": s.Sent, "sampledAt": s.At})
	}
	core.JSON(w, map[string]any{"interfaces": out, "rxBytesPerSecond": rx, "txBytesPerSecond": tx, "available": len(out) > 0})
}
