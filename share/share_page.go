package share

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/rajibdpi/trynas/core"
	qrcode "github.com/skip2/go-qrcode"
)

type shareBrandingKey struct{}

func shareBranding(r *http.Request) core.AppSettings {
	settings, ok := r.Context().Value(shareBrandingKey{}).(core.AppSettings)
	if !ok {
		settings.AppName = "TryNAS"
	}
	return settings
}

const shareBrandTemplate = `{{define "brand"}}<div class="brand"><span class="brand-mark">{{if .LogoURL}}<img src="{{.LogoURL}}" alt="" referrerpolicy="no-referrer">{{else}}<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect width="20" height="8" x="2" y="14" rx="2"/><path d="M6 18h.01M10 18h.01M2 16l3.45-9.88A2 2 0 0 1 7.34 5h9.32a2 2 0 0 1 1.89 1.12L22 16"/></svg>{{end}}</span><span>{{.AppName}}</span></div>{{end}}`

const shareBrandStyle = `.brand{display:flex;align-items:center;justify-content:center;gap:8px;color:var(--primary);font-size:20px;font-weight:700}.brand-mark{display:grid;place-items:center;flex-shrink:0;width:40px;height:40px;border-radius:12px;background:linear-gradient(145deg,color-mix(in srgb,var(--primary) 75%,white),var(--primary));color:var(--primary-foreground,white);box-shadow:0 5px 16px color-mix(in srgb,var(--primary) 22%,transparent)}.brand-mark img{display:block;width:32px;height:32px;object-fit:contain;margin:0;border-radius:0;background:transparent}.brand-mark svg{width:20px;height:20px}`

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
			items = append(items, core.SharedPageItem{Name: name, Kind: kind, Size: size, DownloadURL: "?download=1&item=" + url.QueryEscape(p), Folder: st.IsDir()})
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
	if err := SharePageTemplate.Execute(w, core.SharedPageData{Branding: shareBranding(r), Name: name, Size: detail, Kind: kind, Download: download, Items: items, Truncated: truncated, RelockOnReturn: r.Method == http.MethodPost}); err != nil {
		return
	}
}

var SharePageTemplate = template.Must(template.New("share").Funcs(template.FuncMap{"fileIcon": sharedItemIcon}).Parse(shareBrandTemplate + `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Kind}} · {{.Branding.AppName}}</title>
<style>` + shareBrandStyle + `
:root{color-scheme:light dark;--bg:#f4f6fa;--card:#fff;--text:#192338;--muted:#758097;--border:#e5e9f1;--primary:#be123c}
@media(prefers-color-scheme:dark){:root{--bg:#101522;--card:#181f2f;--text:#e8edf6;--muted:#99a5bc;--border:#2b354a;--primary:#fb7185}}
*{box-sizing:border-box}body{margin:0;min-height:100vh;display:grid;place-items:center;padding:24px;background:var(--bg);color:var(--text);font:15px/1.5 system-ui,sans-serif}
main{width:100%;max-width:720px;padding:32px;border:1px solid var(--border);border-radius:24px;background:var(--card);color:var(--card-text,var(--text));box-shadow:0 18px 70px #171c3410;text-align:center}
.brand{color:var(--primary);font-size:20px;font-weight:700}h1{font-size:24px;overflow-wrap:anywhere;margin:20px 0 8px}p{color:var(--muted)}img{display:block;width:220px;max-width:100%;height:auto;margin:24px auto;border-radius:12px;background:white}
.actions{display:flex;flex-wrap:wrap;gap:12px;justify-content:center}a{display:inline-flex;align-items:center;gap:8px;padding:11px 16px;border:1px solid var(--border);border-radius:10px;text-decoration:none;color:var(--card-text,var(--text));font-weight:600}a.primary{background:var(--primary);color:var(--primary-foreground,white);border-color:var(--primary)}a:hover{opacity:.85}a:focus-visible{outline:3px solid var(--primary);outline-offset:3px}svg{width:18px;height:18px}
.items{text-align:left;margin:24px 0;border:1px solid var(--border);border-radius:12px;max-height:420px;overflow:auto}.item{display:flex;align-items:center;gap:12px;padding:12px;border-bottom:1px solid var(--border)}.item:last-child{border-bottom:0}.item-info{flex:1;min-width:0}.item-name{overflow-wrap:anywhere;font-weight:600}.item-meta{font-size:12px;color:var(--muted)}.item-thumbnail{display:grid;place-items:center;width:48px;height:48px;flex-shrink:0;border-radius:12px;color:var(--icon-color,var(--primary));background:color-mix(in srgb,currentColor 10%,transparent)}.item-thumbnail svg{width:28px;height:28px}.icon-teal{--icon-color:#0d9488}.icon-orange{--icon-color:#ea580c}.icon-violet{--icon-color:#7c3aed}.icon-amber{--icon-color:#d97706}.icon-cyan{--icon-color:#0e7490}.icon-slate{--icon-color:#64748b}.icon-pink{--icon-color:#db2777}.icon-purple{--icon-color:#9333ea}.icon-red{--icon-color:#dc2626}.icon-blue{--icon-color:#2563eb}.icon-emerald{--icon-color:#059669}.icon-yellow{--icon-color:#a16207}.icon-indigo{--icon-color:#4f46e5}.item a{padding:7px 10px;font-size:12px}
</style><script src="/api/branding/theme.js" defer></script>{{if .RelockOnReturn}}<script>
// Browser history must not restore an unlocked page or replay a password POST.
const navigationType = performance.getEntriesByType('navigation')[0]?.type;
if (navigationType === 'reload' || navigationType === 'back_forward') {
  window.location.replace(window.location.pathname);
}
window.addEventListener('pagehide', () => { document.querySelector('main').hidden = true; });
window.addEventListener('pageshow', event => {
  if (event.persisted) window.location.replace(window.location.pathname);
});
</script>{{end}}</head><body><main>
{{template "brand" .Branding}}<h1>{{.Name}}</h1><p>{{.Kind}} · {{.Size}}</p>
<section aria-label="Shared files and folders" class="items">{{range .Items}}<div class="item">{{$icon := fileIcon .Name .Folder}}<span class="item-thumbnail icon-{{$icon.Color}}" title="{{$icon.Label}}"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" role="img" aria-label="{{$icon.Label}}"><path d="{{$icon.Path}}"/></svg></span><div class="item-info"><div class="item-name">{{.Name}}</div><div class="item-meta">{{.Kind}} · {{.Size}}</div></div><a href="{{.DownloadURL}}" aria-label="Download {{.Name}}">Download{{if .Folder}} ZIP{{end}}</a></div>{{end}}</section>
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
