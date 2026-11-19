package application

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"despacho/app/domain"
)

// RenameStatus is the outcome of planning or applying one rename.
type RenameStatus string

const (
	RenameReady    RenameStatus = "READY"
	RenameSame     RenameStatus = "SAME"     // already has the target name
	RenameMissing  RenameStatus = "MISSING"  // source file does not exist
	RenameConflict RenameStatus = "CONFLICT" // target taken, or two rows want it
	RenameInvalid  RenameStatus = "INVALID"
	RenameDone     RenameStatus = "RENAMED"
	RenameFailed   RenameStatus = "FAILED"
)

// RenameOp is one planned file rename. From and To are absolute paths.
type RenameOp struct {
	Row    int
	From   string
	To     string
	Status RenameStatus
	Detail string
}

// extVariable is available in rename rules on top of the table columns:
// {{ext}} is the source file's extension, dot included.
const extVariable = "ext"

// PlanRename computes, without touching the disk, how every file named in
// sourceColumn would be renamed according to rule (e.g.
// "CERT-{{documento_id}}_{{nombre}}{{ext}}"). Showing the plan first and
// applying it as a separate step is what makes a bulk rename safe.
func PlanRename(table domain.Table, sourceColumn, rule, folder string) ([]RenameOp, error) {
	cols := columnIndex(table.Headers)
	srcIdx, ok := cols[domain.NormalizeKey(sourceColumn)]
	if !ok {
		return nil, fmt.Errorf("elige la columna con el nombre actual de cada archivo")
	}
	if strings.TrimSpace(rule) == "" {
		return nil, errors.New("escribe la regla del nuevo nombre")
	}
	cols[extVariable] = -1
	if err := checkVariables(domain.Variables(rule), cols, "la regla de renombrado"); err != nil {
		return nil, err
	}
	if folder == "" {
		return nil, errors.New("elige la carpeta de documentos")
	}

	ops := make([]RenameOp, 0, len(table.Rows))
	for i, row := range table.Rows {
		op := RenameOp{Row: i + 2}
		source := ""
		if srcIdx < len(row) {
			source = strings.TrimSpace(row[srcIdx])
		}
		if source == "" {
			op.Status, op.Detail = RenameInvalid, "Sin archivo origen"
			ops = append(ops, op)
			continue
		}
		op.From = source
		if !filepath.IsAbs(source) {
			op.From = filepath.Join(folder, source)
		}
		if info, err := os.Stat(op.From); err != nil || !info.Mode().IsRegular() {
			op.Status, op.Detail = RenameMissing, "No existe"
			ops = append(ops, op)
			continue
		}

		fields := map[string]string{extVariable: filepath.Ext(op.From)}
		for j, h := range table.Headers {
			if j < len(row) {
				fields[domain.NormalizeKey(h)] = row[j]
			}
		}
		name := SanitizeFileName(domain.Fill(rule, fields, false))
		if name == "" {
			op.Status, op.Detail = RenameInvalid, "El nombre resultante está vacío"
			ops = append(ops, op)
			continue
		}
		if filepath.Ext(name) == "" {
			name += filepath.Ext(op.From) // never drop the extension by accident
		}
		op.To = filepath.Join(filepath.Dir(op.From), name)
		op.Status = RenameReady
		if strings.EqualFold(op.From, op.To) {
			op.Status, op.Detail = RenameSame, "Ya tiene ese nombre"
		}
		ops = append(ops, op)
	}

	markConflicts(ops)
	return ops, nil
}

// markConflicts flags targets claimed by more than one row, and targets that
// already exist on disk (and are not themselves being renamed away). Paths
// are compared case-insensitively because NTFS is case-insensitive.
func markConflicts(ops []RenameOp) {
	claims := map[string]int{}
	sources := map[string]bool{}
	for _, op := range ops {
		if op.Status == RenameReady {
			claims[strings.ToLower(op.To)]++
			sources[strings.ToLower(op.From)] = true
		}
	}
	for i := range ops {
		op := &ops[i]
		if op.Status != RenameReady {
			continue
		}
		key := strings.ToLower(op.To)
		if claims[key] > 1 {
			op.Status, op.Detail = RenameConflict, "Otra fila genera el mismo nombre"
			continue
		}
		if _, err := os.Stat(op.To); err == nil && !sources[key] {
			op.Status, op.Detail = RenameConflict, "Ya existe un archivo con ese nombre"
		}
	}
}

// ApplyRename executes every READY op and returns the updated list. Ops in
// other states are returned untouched, so the result can be shown as-is.
func ApplyRename(ops []RenameOp) []RenameOp {
	out := make([]RenameOp, len(ops))
	copy(out, ops)
	for i := range out {
		op := &out[i]
		if op.Status != RenameReady {
			continue
		}
		// Re-check right before renaming: the folder may have changed since
		// the plan was shown, and os.Rename silently replaces on Windows.
		if _, err := os.Stat(op.To); err == nil {
			op.Status, op.Detail = RenameConflict, "Ya existe un archivo con ese nombre"
			continue
		}
		if err := os.Rename(op.From, op.To); err != nil {
			op.Status, op.Detail = RenameFailed, err.Error()
			continue
		}
		op.Status = RenameDone
	}
	return out
}

// UndoRename reverts every RENAMED op, newest first.
func UndoRename(applied []RenameOp) []RenameOp {
	var undone []RenameOp
	for i := len(applied) - 1; i >= 0; i-- {
		op := applied[i]
		if op.Status != RenameDone {
			continue
		}
		back := RenameOp{Row: op.Row, From: op.To, To: op.From, Status: RenameReady}
		undone = append(undone, ApplyRename([]RenameOp{back})...)
	}
	return undone
}

// SanitizeFileName replaces characters Windows forbids in file names and
// trims the trailing dots/spaces Explorer would silently strip.
func SanitizeFileName(name string) string {
	clean := strings.Map(func(r rune) rune {
		if strings.ContainsRune(`<>:"/\|?*`, r) || r < 32 {
			return '_'
		}
		return r
	}, name)
	return strings.TrimRight(strings.TrimSpace(clean), ". ")
}
