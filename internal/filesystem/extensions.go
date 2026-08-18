package filesystem

import (
	"path/filepath"
	"strings"
)

var mediaExtensions = map[string]bool{
	".mkv":  true,
	".mp4":  true,
	".avi":  true,
	".m4v":  true,
	".mpg":  true,
	".mpeg": true,
	".wmv":  true,
	".ts":   true,
	".m2ts": true,
	".flv":  true,
	".mov":  true,
	".webm": true,
}

func isMediaExt(name string) bool {
	return mediaExtensions[strings.ToLower(filepath.Ext(name))]
}
