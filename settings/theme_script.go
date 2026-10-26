package settings

import "net/http"

// ThemeScript applies the same saved appearance to server-rendered share pages.
func (a *Module) ThemeScript(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write([]byte(`(async () => {
 try {
 const response = await fetch('/api/branding/theme', {cache: 'no-store'});
 if (!response.ok) return;
 const settings = await response.json();
 const media = matchMedia('(prefers-color-scheme: dark)');
 let override;
 try { override = localStorage.getItem('trynas-theme'); } catch {}
 const contrast = color => {
 const channels = color.slice(1).match(/../g).map(value => {
 const c = parseInt(value, 16) / 255;
 return c <= .04045 ? c / 12.92 : ((c + .055) / 1.055) ** 2.4;
 });
 return channels[0] * .2126 + channels[1] * .7152 + channels[2] * .0722 > .179 ? '#000000' : '#ffffff';
 };
 const update = () => {
 const mode = override === 'light' || override === 'dark' ? override : settings.themeMode || 'system';
 const dark = mode === 'system' ? media.matches : mode === 'dark';
 const primary = (dark ? settings.darkPrimary : settings.lightPrimary) || (dark ? '#fb7185' : '#be123c');
 const bg = (dark ? settings.darkBackground : settings.lightBackground) || (dark ? '#101522' : '#f4f6fa');
 const card = (dark ? settings.darkCard : settings.lightCard) || (dark ? '#181f2f' : '#ffffff');
 const values = {primary, bg, card, text: contrast(bg), 'card-text': contrast(card), 'primary-foreground': contrast(primary), muted: 'color-mix(in srgb, ' + contrast(bg) + ' 65%, ' + bg + ')', border: 'color-mix(in srgb, ' + contrast(card) + ' 15%, ' + card + ')'};
 for (const [key, value] of Object.entries(values)) document.documentElement.style.setProperty('--' + key, value);
 document.documentElement.style.colorScheme = dark ? 'dark' : 'light';
 };
 update();
 media.addEventListener('change', update);
 } catch {}
})();`))
}
