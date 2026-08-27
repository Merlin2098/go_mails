// Package filesystem resolves the association between an Employee (by DNI)
// and the Document file that belongs to them, per the rule fixed in
// docs/specs/docs_enrutamiento.md §5: document_id = file.stem, independent
// of extension.
package filesystem

import (
	"os"
	"path/filepath"
	"strings"
)

// ResolveError is returned when a DNI cannot be resolved to exactly one
// file — either none found or more than one candidate (ambiguous).
type ResolveError struct {
	DNI     string
	Reason  string
	Matches []string // populated only when the error is ambiguity
}

func (e *ResolveError) Error() string {
	return e.Reason
}

// Index maps a DNI (file stem) to its resolved absolute path, built once per
// folder so resolving many employees does not re-scan the directory.
type Index struct {
	byStem map[string][]string // stem -> absolute paths of every file sharing it
}

// BuildIndex scans folderPath (non-recursive — matches context.md §4.2,
// documents live flat in the selected folder) and groups files by stem.
func BuildIndex(folderPath string) (Index, error) {
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return Index{}, err
	}

	byStem := make(map[string][]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		stem := strings.TrimSuffix(name, filepath.Ext(name))
		fullPath := filepath.Join(folderPath, name)
		byStem[stem] = append(byStem[stem], fullPath)
	}
	return Index{byStem: byStem}, nil
}

// Resolve returns the single document path matching dni, or a ResolveError
// describing why it could not (not found, or ambiguous — multiple files
// sharing the same stem with different extensions).
func (idx Index) Resolve(dni string) (string, error) {
	matches := idx.byStem[dni]
	switch len(matches) {
	case 0:
		return "", &ResolveError{
			DNI:    dni,
			Reason: "Documento no encontrado",
		}
	case 1:
		return matches[0], nil
	default:
		return "", &ResolveError{
			DNI:     dni,
			Reason:  "Documento ambiguo: múltiples archivos coinciden con este DNI",
			Matches: matches,
		}
	}
}
