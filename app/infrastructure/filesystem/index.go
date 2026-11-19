// Package filesystem scans the documents folder once and answers "which
// files belong to this recipient?" without touching the disk again per row.
package filesystem

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Index is a snapshot of the files inside a folder, built once per
// validation. Matching is case-insensitive because Windows file names are:
// "CERT-1001.PDF" and "cert-1001.pdf" are the same file to the user.
type Index struct {
	root  string
	files []string // paths relative to root, with OS separators
}

// BuildIndex lists every regular file under root. With recursive=false only
// the top level is scanned.
func BuildIndex(root string, recursive bool) (Index, error) {
	idx := Index{root: root}
	if !recursive {
		entries, err := os.ReadDir(root)
		if err != nil {
			return Index{}, err
		}
		for _, e := range entries {
			if e.Type().IsRegular() {
				idx.files = append(idx.files, e.Name())
			}
		}
		return idx, nil
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			idx.files = append(idx.files, rel)
		}
		return nil
	})
	if err != nil {
		return Index{}, err
	}
	return idx, nil
}

// Glob returns the absolute paths of every indexed file whose base name
// matches pattern (filepath.Match syntax: *, ?, [abc]). Matching on the
// base name keeps patterns simple even when recursive scanning is enabled.
func (idx Index) Glob(pattern string) ([]string, error) {
	pattern = strings.ToLower(pattern)
	// Validate the pattern once so a malformed one fails loudly instead of
	// silently matching nothing.
	if _, err := filepath.Match(pattern, ""); err != nil {
		return nil, err
	}
	var matches []string
	for _, rel := range idx.files {
		ok, _ := filepath.Match(pattern, strings.ToLower(filepath.Base(rel)))
		if ok {
			matches = append(matches, filepath.Join(idx.root, rel))
		}
	}
	return matches, nil
}

// Resolve turns a file name taken from the spreadsheet into an absolute
// path. Absolute paths are checked on disk as-is; relative ones are looked
// up in the index (case-insensitively).
func (idx Index) Resolve(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", false
	}
	if filepath.IsAbs(name) {
		info, err := os.Stat(name)
		return name, err == nil && info.Mode().IsRegular()
	}
	want := strings.ToLower(filepath.Clean(name))
	for _, rel := range idx.files {
		if strings.ToLower(rel) == want {
			return filepath.Join(idx.root, rel), true
		}
	}
	return "", false
}

// Root is the folder this index was built from.
func (idx Index) Root() string { return idx.root }
