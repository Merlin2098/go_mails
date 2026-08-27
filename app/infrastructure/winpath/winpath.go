// Package winpath handles the Windows MAX_PATH (260 character) limitation
// for the one API in this project that does not transparently support long
// paths: Outlook COM (Attachments.Add). Go's own os.* functions have
// supported long paths automatically since Go 1.19 and need no help here —
// this package exists specifically for the COM boundary.
package winpath

import (
	"path/filepath"
	"strings"
)

// MaxPath is the classic Windows path length limit that APIs without long-path
// support still enforce.
const MaxPath = 260

// ExtendedLengthPrefix converts an absolute Windows path to its "extended-length"
// form (\\?\C:\... or \\?\UNC\server\share\... for UNC paths), which tells
// Windows APIs to bypass MAX_PATH and skip automatic path normalization.
// Because normalization is skipped, path must already be clean (no . or ..
// segments) — callers should pass the result of filepath.Clean or
// filepath.Abs, never a raw user-supplied path.
//
// Returns path unchanged if it is not absolute, already carries the prefix,
// or is not long enough to need it — the prefix is only ever necessary past
// MaxPath, and applying it unconditionally would make already-working short
// paths harder to read in logs and error messages for no benefit.
func ExtendedLengthPrefix(path string) string {
	if len(path) < MaxPath || strings.HasPrefix(path, `\\?\`) {
		return path
	}
	if !filepath.IsAbs(path) {
		return path
	}

	clean := filepath.Clean(path)
	if strings.HasPrefix(clean, `\\`) {
		// UNC path: \\server\share\... -> \\?\UNC\server\share\...
		return `\\?\UNC\` + strings.TrimPrefix(clean, `\\`)
	}
	return `\\?\` + clean
}
