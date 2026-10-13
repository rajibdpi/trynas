package core

import "strings"

// ImageContentType supplies consistent image types independent of host MIME configuration.
func ImageContentType(extension string) string {
	switch strings.ToLower(strings.TrimPrefix(extension, ".")) {
	case "jpg", "jpeg", "jpe", "jfif", "pjpeg", "pjp":
		return "image/jpeg"
	case "png", "apng":
		return "image/png"
	case "gif":
		return "image/gif"
	case "webp":
		return "image/webp"
	case "svg", "svgz":
		return "image/svg+xml"
	case "ico", "cur":
		return "image/x-icon"
	case "bmp", "dib":
		return "image/bmp"
	case "avif":
		return "image/avif"
	case "tif", "tiff":
		return "image/tiff"
	case "heic":
		return "image/heic"
	case "heif":
		return "image/heif"
	case "jxl":
		return "image/jxl"
	default:
		return ""
	}
}
