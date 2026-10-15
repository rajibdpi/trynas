package share

import (
	"path"
	"strings"
)

type shareFileIcon struct {
	Extensions string
	Label      string
	Color      string
	Path       string
}

// Keep extension categories aligned with frontend/modules/files/file-type-icon.tsx.
var shareFileIcons = []shareFileIcon{
	{" db sqlite sqlite3 mdb accdb db3 parquet ", "Database", "teal", `M12 8c5 0 9-1.3 9-3s-4-3-9-3-9 1.3-9 3 4 3 9 3z M3 5v14c0 4 18 4 18 0V5 M3 12c0 4 18 4 18 0`},
	{" epub mobi azw azw3 fb2 ", "E-book", "orange", `M12 7v14 M3 3c4-1 7 0 9 2 2-2 5-3 9-2v16c-4-1-7 0-9 2-2-2-5-3-9-2z`},
	{" ttf otf woff woff2 eot ", "Font", "violet", `M4 4h16 M12 4v16 M8 20h8`},
	{" pem crt cer der p12 pfx pub ", "Certificate / key", "amber", `M14 2H6v20h12V6z M14 2v5h4 M10 12a2 2 0 1 0 0 4 2 2 0 0 0 0-4 M10 16v3`},
	{" obj stl fbx gltf glb blend 3ds step stp iges igs dwg dxf ", "3D / CAD model", "cyan", `m12 3 9 5v9l-9 5-9-5V8z M3 8l9 5 9-5 M12 13v9`},
	{" iso img vhd vhdx vmdk qcow2 ", "Disk image", "slate", `M21 12a9 9 0 1 0-18 0 9 9 0 0 0 18 0 M14 12a2 2 0 1 0-4 0 2 2 0 0 0 4 0 M7 7l2 2 M15 15l2 2`},
	{" jpg jpeg jpe jfif pjpeg pjp png apng gif webp svg svgz avif bmp dib ico cur tif tiff heic heif jxl raw arw cr2 cr3 nef nrw dng orf rw2 pef raf srw psd psb tga dds ", "Image", "pink", `M14 2H6v20h12V6z M14 2v5h4 M6 18l4-5 3 3 2-2 3 4 M10 9h.01`},
	{" mp4 mkv mov avi webm m4v mpeg mpg wmv flv ogv 3gp 3g2 mts m2ts vob mxf ", "Video", "purple", `M14 2H6v20h12V6z M14 2v5h4 m-8 4 5 3-5 3z`},
	{" mp3 wav ogg flac aac m4a wma opus aiff aif alac mid midi amr mka ", "Audio", "amber", `M14 2H6v20h12V6z M14 2v5h4 M12 17V10l3 1 M12 17c0 3-5 3-5 0s5-3 5 0`},
	{" pdf ", "PDF document", "red", `M14 2H6v20h12V6z M14 2v5h4 M9 12h6 M9 16h6`},
	{" doc docx odt rtf txt md markdown log rst tex pages text ", "Text document", "blue", `M14 2H6v20h12V6z M14 2v5h4 M9 12h6 M9 16h6`},
	{" xls xlsx csv ods tsv ", "Spreadsheet", "emerald", `M14 2H6v20h12V6z M14 2v5h4 M6 12h12 M6 16h12 M11 8v14`},
	{" ppt pptx odp key ", "Presentation", "orange", `M3 3h18v13H3z M12 16v5 M8 21l4-3 4 3 M7 11l3-3 3 2 4-4`},
	{" zip rar 7z tar gz bz2 xz tgz zst gzip bzip2 tbz tbz2 txz lz lzma cab ", "Archive", "yellow", `M14 2H6v20h12V6z M14 2v5h4 M10 3v2m0 2v2m0 2v2 M9 16h3v3H9z`},
	{" js jsx ts tsx py go html htm css scss json xml sql sh bash c cpp h rs java php rb vue svelte mjs cjs ps1 bat cmd zsh fish ksh cc cxx hpp cs kt kts swift dart lua r pl ex exs erl hs ipynb less sass graphql gql wasm ", "Code", "indigo", `M14 2H6v20h12V6z M14 2v5h4 m-8 5-3 3 3 3 m4-6 3 3-3 3`},
	{" exe msi dmg app apk deb rpm appimage msix aab ipa pkg ", "Application / installer", "cyan", `M14 2H6v20h12V6z M14 2v5h4 M14 15a3 3 0 1 0-6 0 3 3 0 0 0 6 0 M11 10v2m0 6v2m-5-5h2m6 0h2`},
	{" env ini conf config toml yaml yml properties cfg editorconfig gitignore dockerignore npmrc ", "Configuration", "slate", `M14 2H6v20h12V6z M14 2v5h4 M9 11h6 M9 16h6 M11 10v2 M13 15v2`},
}

func sharedItemIcon(name string, folder bool) shareFileIcon {
	if folder {
		return shareFileIcon{Label: "Folder", Color: "blue", Path: "M3 7V4h6l2 3h10v13H3z"}
	}
	filename := strings.ToLower(path.Base(name))
	extension := strings.TrimPrefix(path.Ext(filename), ".")
	if filename == "dockerfile" || filename == "makefile" {
		extension = "go"
	}
	if strings.HasPrefix(filename, ".env.") {
		extension = "env"
	}
	if extension != "" {
		for _, icon := range shareFileIcons {
			if strings.Contains(icon.Extensions, " "+extension+" ") {
				return icon
			}
		}
	}
	return shareFileIcon{Label: "File", Color: "default", Path: "M14 2H6v20h12V6z M14 2v5h4"}
}
