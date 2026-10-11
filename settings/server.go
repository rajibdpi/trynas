package settings

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
	"github.com/rajibdpi/trynas/core"
)

func (a *Module) SaveServerConfiguration(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(core.UserKey).(core.User)
	if user.Role != "admin" {
		core.Bad(w, "administrator required", 403)
		return
	}
	var input struct {
		StorageRoot  string `json:"storageRoot"`
		Address      string `json:"address"`
		CorsOrigin   string `json:"corsOrigin"`
		CookieSecure bool   `json:"cookieSecure"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil {
		core.Bad(w, "invalid server configuration", 400)
		return
	}
	input.StorageRoot = strings.TrimSpace(input.StorageRoot)
	input.Address = strings.TrimSpace(input.Address)
	input.CorsOrigin = strings.TrimSpace(input.CorsOrigin)
	if input.StorageRoot == "" || strings.ContainsAny(input.StorageRoot, "\r\n\x00") {
		core.Bad(w, "choose a storage root", 400)
		return
	}
	_, port, err := net.SplitHostPort(input.Address)
	if err != nil || port == "" {
		core.Bad(w, "listen address must be host:port", 400)
		return
	}
	if _, err := net.LookupPort("tcp", port); err != nil {
		core.Bad(w, "invalid listen port", 400)
		return
	}
	origins := []string{}
	seen := map[string]bool{}
	for _, value := range strings.FieldsFunc(input.CorsOrigin, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' }) {
		origin, err := url.Parse(strings.TrimSpace(value))
		if err != nil || (origin.Scheme != "http" && origin.Scheme != "https") || origin.Host == "" || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" || (origin.Path != "" && origin.Path != "/") {
			core.Bad(w, "each allowed frontend origin must be an HTTP(S) origin", 400)
			return
		}
		normalized := origin.Scheme + "://" + origin.Host
		if !seen[normalized] {
			seen[normalized] = true
			origins = append(origins, normalized)
		}
	}
	if len(origins) == 0 || len(origins) > 20 {
		core.Bad(w, "provide 1–20 allowed frontend origins", 400)
		return
	}
	input.CorsOrigin = strings.Join(origins, ",")
	configFile := core.Env("TRYNAS_CONFIG_FILE", ".env")
	values := map[string]string{}
	if existing, err := godotenv.Read(configFile); err == nil {
		values = existing
	} else if !os.IsNotExist(err) {
		core.Bad(w, "could not read configuration file", 500)
		return
	}
	values["STORAGE_ROOT"] = input.StorageRoot
	values["ADDR"] = input.Address
	values["CORS_ORIGIN"] = input.CorsOrigin
	values["COOKIE_SECURE"] = "false"
	if input.CookieSecure {
		values["COOKIE_SECURE"] = "true"
	}
	data, err := godotenv.Marshal(values)
	if err != nil {
		core.Bad(w, "could not serialize configuration", 500)
		return
	}
	temp, err := os.CreateTemp(filepath.Dir(configFile), ".trynas-config-")
	if err != nil {
		core.Bad(w, "configuration file is not writable", 500)
		return
	}
	defer os.Remove(temp.Name())
	if err = temp.Chmod(0600); err == nil {
		_, err = temp.WriteString(data + "\n")
	}
	if err == nil {
		err = temp.Sync()
	}
	closeErr := temp.Close()
	if err != nil || closeErr != nil {
		core.Bad(w, "could not save configuration", 500)
		return
	}
	if err := os.Rename(temp.Name(), configFile); err != nil {
		core.Bad(w, "could not replace configuration file", 500)
		return
	}
	a.Log(user.ID, "settings", "server", user.IP)
	core.JSON(w, map[string]bool{"restartRequired": true})
}
