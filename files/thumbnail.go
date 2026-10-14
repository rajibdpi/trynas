package files

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"github.com/rajibdpi/trynas/core"
	"html"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

func (a *Module) Thumbnail(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(core.UserKey).(core.User)
	p := r.URL.Query().Get("path")
	if !a.Can(u, p, false) {
		core.Bad(w, "access denied", 403)
		return
	}
	full, err := core.Safe(a.Root, p)
	if err != nil {
		core.Bad(w, "invalid path", 400)
		return
	}
	file, err := os.Open(full)
	if err != nil {
		core.Bad(w, "file not found", 404)
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		core.Bad(w, "select a file", 400)
		return
	}
	ext := strings.ToLower(filepath.Ext(p))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox")
	w.Header().Set("Cache-Control", "private, no-cache")
	if info.Size() <= 20<<20 && (ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".gif") {
		config, _, err := image.DecodeConfig(file)
		if err == nil && config.Width > 0 && config.Height > 0 && int64(config.Width)*int64(config.Height) <= 32000000 {
			_, _ = file.Seek(0, 0)
			if source, _, err := image.Decode(file); err == nil {
				width, height := config.Width, config.Height
				if width > height && width > 160 {
					height = max(1, height*160/width)
					width = 160
				} else if height > 160 {
					width = max(1, width*160/height)
					height = 160
				}
				target := image.NewRGBA(image.Rect(0, 0, width, height))
				draw.Draw(target, target.Bounds(), image.NewUniform(image.White), image.Point{}, draw.Src)
				bounds := source.Bounds()
				for y := 0; y < height; y++ {
					for x := 0; x < width; x++ {
						target.Set(x, y, source.At(bounds.Min.X+x*config.Width/width, bounds.Min.Y+y*config.Height/height))
					}
				}
				w.Header().Set("Content-Type", "image/png")
				_ = png.Encode(w, target)
				return
			}
		}
	}
	_, _ = file.Seek(0, 0)
	text := ""
	if strings.Contains("|.txt|.md|.markdown|.log|.csv|.tsv|.json|.xml|.yaml|.yml|.toml|.ini|.conf|.env|.js|.jsx|.ts|.tsx|.py|.go|.rs|.java|.html|.css|.scss|.sql|.sh|.c|.h|.cpp|", "|"+ext+"|") || ext == "" {
		data, _ := io.ReadAll(io.LimitReader(file, 16384))
		if utf8.Valid(data) && !bytes.ContainsRune(data, 0) {
			text = string(data)
		}
	}
	if info.Size() <= 100<<20 && (ext == ".docx" || ext == ".xlsx" || ext == ".pptx" || ext == ".odt" || ext == ".ods" || ext == ".odp" || ext == ".zip") {
		if archive, err := zip.NewReader(file, info.Size()); err == nil && len(archive.File) <= 10000 {
			for _, entry := range archive.File {
				if entry.Name == "docProps/thumbnail.jpeg" || entry.Name == "docProps/thumbnail.png" || entry.Name == "Thumbnails/thumbnail.png" {
					if entry.UncompressedSize64 > 1<<20 {
						continue
					}
					reader, err := entry.Open()
					if err != nil {
						continue
					}
					data, err := io.ReadAll(io.LimitReader(reader, (1<<20)+1))
					_ = reader.Close()
					if err == nil && len(data) <= 1<<20 {
						if _, format, err := image.DecodeConfig(bytes.NewReader(data)); err == nil && (format == "png" || format == "jpeg") {
							mime := "image/png"
							if format == "jpeg" {
								mime = "image/jpeg"
							}
							w.Header().Set("Content-Type", mime)
							_, _ = w.Write(data)
							return
						}
					}
				}
				if ext == ".docx" && entry.Name == "word/document.xml" {
					reader, err := entry.Open()
					if err != nil {
						continue
					}
					decoder := xml.NewDecoder(io.LimitReader(reader, 65536))
					var excerpt strings.Builder
					for excerpt.Len() < 2000 {
						token, err := decoder.Token()
						if err != nil {
							break
						}
						if element, ok := token.(xml.StartElement); ok && element.Name.Local == "t" {
							var value string
							if decoder.DecodeElement(&value, &element) == nil {
								excerpt.WriteString(value)
								excerpt.WriteByte(' ')
							}
						}
					}
					_ = reader.Close()
					text = excerpt.String()
				}
			}
			if ext == ".zip" {
				for i, entry := range archive.File {
					if i >= 7 {
						break
					}
					text += entry.Name + "\n"
				}
			}
		}
	}
	// Let the web client use its file-type icon when no real preview exists.
	if r.URL.Query().Get("fallback") == "icon" && strings.TrimSpace(text) == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	label := strings.ToUpper(strings.TrimPrefix(ext, "."))
	if label == "" {
		label = "FILE"
	}
	var svg strings.Builder
	svg.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 160 160"><rect width="160" height="160" rx="12" fill="#f8fafc"/><path d="M25 16h90l20 20v108H25z" fill="white" stroke="#cbd5e1"/><path d="M115 16v20h20" fill="none" stroke="#cbd5e1"/>`)
	fmt.Fprintf(&svg, `<text x="36" y="46" font-family="sans-serif" font-size="14" font-weight="bold" fill="#475569">%s</text>`, html.EscapeString(string([]rune(label)[:min(10, len([]rune(label)))])))
	scanner := bufio.NewScanner(strings.NewReader(text))
	line := 0
	for scanner.Scan() && line < 7 {
		value := []rune(strings.TrimSpace(scanner.Text()))
		if len(value) == 0 {
			continue
		}
		fmt.Fprintf(&svg, `<text x="36" y="%d" font-family="monospace" font-size="8" fill="#64748b">%s</text>`, 64+line*10, html.EscapeString(string(value[:min(len(value), 18)])))
		line++
	}
	if line == 0 {
		svg.WriteString(`<path d="M38 68h65M38 82h80M38 96h58M38 110h70" stroke="#cbd5e1" stroke-width="4" stroke-linecap="round"/>`)
	}
	svg.WriteString(`</svg>`)
	w.Header().Set("Content-Type", "image/svg+xml")
	_, _ = io.WriteString(w, svg.String())
}
