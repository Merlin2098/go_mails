// Package excel isolates all interaction with the input spreadsheet. The
// domain and application layers only see []domain.Employee and
// domain.MessageTemplate — never *excelize.File, sheet names, or cell
// references.
package excel

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"

	"boleta-mailer/app/domain"
)

// Sheets are addressed by position, not by name: the first sheet is always
// the recipient table, the second is always the message template. This
// survives the user renaming sheets (observed in practice: "Hoja1/Hoja2" ->
// "Data/Mensaje") without any code change, unlike matching on a literal name.
const (
	dataSheetIndex    = 0
	messageSheetIndex = 1
)

// Required columns on the recipient sheet, matched case-insensitively.
// docs/boleta_mailer_mvp_context.md §10 fixes this contract; docs/specs/
// docs_enrutamiento.md §0.1 drops "ruta_archivo" from it.
const (
	colDNI    = "DNI"
	colNombre = "NOMBRE"
	colCorreo = "CORREO"
)

// Required columns on the message sheet.
const (
	colAsunto  = "Asunto"
	colMensaje = "MENSAJE"
)

// ParsedWorkbook is everything read from the input spreadsheet, before
// resolving documents or validating. It mirrors the domain shapes directly.
type ParsedWorkbook struct {
	Employees []domain.Employee
	Template  domain.MessageTemplate
}

// ReadWorkbook opens the .xlsx at path and parses both the recipient sheet
// and the message template sheet. It returns an error describing exactly
// what is missing (sheet, column) rather than a generic failure — the
// application layer surfaces this directly to the user before any send
// happens (context.md §11, §15).
func ReadWorkbook(path string) (ParsedWorkbook, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return ParsedWorkbook{}, fmt.Errorf("no se pudo abrir el archivo Excel: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) <= messageSheetIndex {
		return ParsedWorkbook{}, fmt.Errorf(
			"el Excel debe tener al menos 2 hojas (destinatarios y mensaje), encontradas: %d",
			len(sheets),
		)
	}

	employees, err := readEmployees(f, sheets[dataSheetIndex])
	if err != nil {
		return ParsedWorkbook{}, err
	}

	template, err := readTemplate(f, sheets[messageSheetIndex])
	if err != nil {
		return ParsedWorkbook{}, err
	}

	return ParsedWorkbook{Employees: employees, Template: template}, nil
}

func readEmployees(f *excelize.File, sheet string) ([]domain.Employee, error) {
	rows, err := f.GetRows(sheet)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer la hoja %q: %w", sheet, err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("la hoja %q está vacía", sheet)
	}

	col, err := columnIndex(rows[0], colDNI, colNombre, colCorreo)
	if err != nil {
		return nil, fmt.Errorf("hoja %q: %w", sheet, err)
	}

	var employees []domain.Employee
	for _, row := range rows[1:] {
		if isBlankRow(row) {
			continue
		}
		employees = append(employees, domain.Employee{
			DNI:   cellAt(row, col[colDNI]),
			Name:  cellAt(row, col[colNombre]),
			Email: cellAt(row, col[colCorreo]),
		})
	}
	return employees, nil
}

func readTemplate(f *excelize.File, sheet string) (domain.MessageTemplate, error) {
	rows, err := f.GetRows(sheet)
	if err != nil {
		return domain.MessageTemplate{}, fmt.Errorf("no se pudo leer la hoja %q: %w", sheet, err)
	}
	if len(rows) < 2 {
		return domain.MessageTemplate{}, fmt.Errorf("la hoja %q debe tener una fila de encabezado y una fila de datos", sheet)
	}

	col, err := columnIndex(rows[0], colAsunto, colMensaje)
	if err != nil {
		return domain.MessageTemplate{}, fmt.Errorf("hoja %q: %w", sheet, err)
	}

	dataRow := rows[1]
	return domain.MessageTemplate{
		Subject: cellAt(dataRow, col[colAsunto]),
		Body:    cellAt(dataRow, col[colMensaje]),
	}, nil
}

// columnIndex maps each required column name (case-insensitive) to its
// position in the header row. Returns an error naming every missing column
// at once, not just the first one found.
func columnIndex(header []string, required ...string) (map[string]int, error) {
	index := make(map[string]int, len(header))
	for i, name := range header {
		index[strings.ToUpper(strings.TrimSpace(name))] = i
	}

	result := make(map[string]int, len(required))
	var missing []string
	for _, name := range required {
		key := strings.ToUpper(name)
		i, ok := index[key]
		if !ok {
			missing = append(missing, name)
			continue
		}
		result[name] = i
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("faltan columnas requeridas: %s", strings.Join(missing, ", "))
	}
	return result, nil
}

func cellAt(row []string, i int) string {
	if i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func isBlankRow(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}
