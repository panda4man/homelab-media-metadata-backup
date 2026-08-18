package match

import (
	"path"
	"strings"
)

// normalizeRelPath makes a relative path comparable regardless of which
// separator style or "./" prefix it arrived with. It does not attempt
// Unicode normalization (e.g. NFC/NFD filename variants) - that is a real
// macOS/HFS+ quirk but out of scope for v1, which stays stdlib-only.
func normalizeRelPath(p string) string {
	p = strings.ReplaceAll(p, `\`, "/")
	p = strings.TrimPrefix(p, "./")
	return path.Clean(strings.Trim(p, "/"))
}

// normalizeRelPathFold is normalizeRelPath with case folded, used as a
// fallback when the exact-case lookup misses.
func normalizeRelPathFold(p string) string {
	return strings.ToLower(normalizeRelPath(p))
}
