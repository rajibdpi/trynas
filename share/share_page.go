package share

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/rajib/trynas/core"
	qrcode "github.com/skip2/go-qrcode"
)

func ServeQR(w http.ResponseWriter, r *http.Request, token string) {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	origin := &url.URL{Scheme: scheme, Host: r.Host}
	// The browser supplies its visible origin so HTTPS reverse proxies work too.
	if value := r.URL.Query().Get("origin"); value != "" {
		parsed, err := url.Parse(value)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			core.Bad(w, "invalid share origin", http.StatusBadRequest)
			return
		}
		origin = parsed
	}
	origin.Path = "/s/" + token
	png, err := qrcode.Encode(origin.String(), qrcode.Medium, 512)
	if err != nil {
		core.Bad(w, "could not generate QR code", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", `attachment; filename="trynas-share-qr.png"`)
	}
	_, _ = w.Write(png)
}

func (a *Module) ListSharedItems(owner core.User, paths []string) ([]core.SharedPageItem, bool, error) {
	var items []core.SharedPageItem
	truncated := false
	for _, root := range paths {
		full, err := core.Safe(a.Root, root)
		if err != nil {
			return nil, false, err
		}
		err = filepath.Walk(full, func(f string, st os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			suffix, err := filepath.Rel(full, f)
			if err != nil {
				return err
			}
			p := root
			if suffix != "." {
				p = core.RelJoin(root, suffix)
			}
			if _, err := core.CleanPath(p); err != nil {
				if st.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if st.Mode()&os.ModeSymlink != 0 {
				return nil
			}
			if st.IsDir() && !a.CanBrowse(owner, p) {
				return filepath.SkipDir
			}
			if !a.Can(owner, p, false) {
				return nil
			}
			if !st.IsDir() && !st.Mode().IsRegular() {
				return nil
			}
			if len(items) >= 1000 {
				truncated = true
				return filepath.SkipAll
			}
			name := filepath.Base(full)
			if root == "" {
				name = "Storage"
			}
			if suffix != "." {
				name += "/" + filepath.ToSlash(suffix)
			}
			kind, size := "File", fmt.Sprintf("%.2f MiB", float64(st.Size())/1048576)
			if st.IsDir() {
				kind, size = "Folder", "ZIP download"
			}
			items = append(items, core.SharedPageItem{name, kind, size, "?download=1&item=" + url.QueryEscape(p), st.IsDir()})
			return nil
		})
		if err != nil {
			return nil, false, err
		}
		if truncated {
			break
		}
	}
	return items, truncated, nil
}

func ServeSharePage(w http.ResponseWriter, r *http.Request, name string, size int64, folder bool, items []core.SharedPageItem, truncated bool, collection ...bool) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	kind, detail, download := "Shared file", fmt.Sprintf("%.2f MiB", float64(size)/1048576), "Download file"
	if folder {
		kind, detail, download = "Shared folder", "Download as ZIP", "Download folder ZIP"
	}
	if len(collection) > 0 && collection[0] {
		kind, detail, download = "Shared collection", "Selected files and folders", "Download all (ZIP)"
	}
	if err := SharePageTemplate.Execute(w, core.SharedPageData{name, detail, kind, download, items, truncated}); err != nil {
		return
	}
}

var SharePageTemplate = template.Must(template.New("share").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Shared file · TryNAS</title>
<style>
:root{color-scheme:light dark;--bg:#f4f6fa;--card:#fff;--text:#192338;--muted:#758097;--border:#e5e9f1;--primary:#6254e8}
@media(prefers-color-scheme:dark){:root{--bg:#101522;--card:#181f2f;--text:#e8edf6;--muted:#99a5bc;--border:#2b354a;--primary:#a69cff}}
*{box-sizing:border-box}body{margin:0;min-height:100vh;display:grid;place-items:center;padding:24px;background:var(--bg);color:var(--text);font:15px/1.5 system-ui,sans-serif}
main{width:100%;max-width:720px;padding:32px;border:1px solid var(--border);border-radius:24px;background:var(--card);box-shadow:0 18px 70px #171c3410;text-align:center}
.brand{color:var(--primary);font-size:20px;font-weight:700}h1{font-size:24px;overflow-wrap:anywhere;margin:20px 0 8px}p{color:var(--muted)}img{display:block;width:220px;max-width:100%;height:auto;margin:24px auto;border-radius:12px;background:white}
.actions{display:flex;flex-wrap:wrap;gap:12px;justify-content:center}a{display:inline-flex;align-items:center;gap:8px;padding:11px 16px;border:1px solid var(--border);border-radius:10px;text-decoration:none;color:var(--text);font-weight:600}a.primary{background:#6254e8;color:white;border-color:#6254e8}a:hover{opacity:.85}a:focus-visible{outline:3px solid var(--primary);outline-offset:3px}svg{width:18px;height:18px}
.items{text-align:left;margin:24px 0;border:1px solid var(--border);border-radius:12px;max-height:420px;overflow:auto}.item{display:flex;align-items:center;gap:12px;padding:12px;border-bottom:1px solid var(--border)}.item:last-child{border-bottom:0}.item-info{flex:1;min-width:0}.item-name{overflow-wrap:anywhere;font-weight:600}.item-meta{font-size:12px;color:var(--muted)}.item-icon{color:var(--primary);flex-shrink:0}.item a{padding:7px 10px;font-size:12px}
</style></head><body><main>
<div class="brand">TryNAS</div><h1>{{.Name}}</h1><p>{{.Kind}} · {{.Size}}</p>
<section aria-label="Shared files and folders" class="items">{{range .Items}}<div class="item"><svg class="item-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">{{if .Folder}}<path d="M3 7V4h6l2 3h10v13H3z"/>{{else}}<path d="M6 3h8l4 4v14H6zM14 3v5h4"/>{{end}}</svg><div class="item-info"><div class="item-name">{{.Name}}</div><div class="item-meta">{{.Kind}} · {{.Size}}</div></div><a href="{{.DownloadURL}}" aria-label="Download {{.Name}}">Download{{if .Folder}} ZIP{{end}}</a></div>{{end}}</section>
{{if .Truncated}}<p>Showing the first 1,000 items. The complete selection is included in the ZIP download.</p>{{end}}
<img id="share-qr" src="?qr=1" alt="QR code for this shared file" width="512" height="512">
<p>Scan to open this shared link.</p>
<div class="actions"><a class="primary" href="?download=1"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M12 3v12m-5-5 5 5 5-5M5 16v5h14v-5"/></svg>{{.Download}}</a>
<a id="download-qr" href="?qr=1&amp;download=1" download="trynas-share-qr.png"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M3 3h6v6H3zM15 3h6v6h-6zM3 15h6v6H3zM15 15h3v3h3v3h-6z"/></svg>Download QR code</a></div>
</main><script>
const origin = encodeURIComponent(window.location.origin);
document.getElementById('share-qr').src = '?qr=1&origin=' + origin;
document.getElementById('download-qr').href = '?qr=1&download=1&origin=' + origin;
</script></body></html>`))
