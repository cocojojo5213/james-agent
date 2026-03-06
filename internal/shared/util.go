package shared

import (
	"strings"
)

// Truncate returns s truncated to n characters with "..." appended if truncated.
func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// NormalizeMediaType extracts the media type from a Content-Type header value,
// stripping parameters like charset.
func NormalizeMediaType(value string) string {
	contentType := strings.TrimSpace(value)
	if contentType == "" {
		return ""
	}
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = contentType[:idx]
	}
	return strings.TrimSpace(contentType)
}
