// Package spreadsheet isolates every interaction with input/output tables
// (.xlsx via excelize, .csv via encoding/csv). The rest of the app only sees
// domain.Table and domain.Template — never *excelize.File or sheet names.
package spreadsheet

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"

	"despacho/app/domain"
)

// messageSheetName is the optional sheet holding a starting subject/body.
const messageSheetName = "mensaje"

// ReadTable reads the recipient table from path. For .xlsx files the first
// sheet is used (sheets are addressed by position so renaming them never
// breaks anything); .csv files may use "," or ";" — the latter is what
// Excel produces in Spanish-locale Windows.
func ReadTable(path string) (domain.Table, error) {
	var rows [][]string
	var err error
	switch strings.ToLower(filepath.Ext(path)) {
	case ".xlsx", ".xlsm":
		rows, err = readXLSXRows(path)
	case ".csv":
		rows, err = readCSVRows(path)
	default:
		return domain.Table{}, fmt.Errorf("formato no soportado %q: usa .xlsx o .csv", filepath.Ext(path))
	}
	if err != nil {
		return domain.Table{}, err
	}
	return toTable(rows)
}

func readXLSXRows(path string) ([][]string, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir el Excel: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("el Excel no tiene hojas")
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer la hoja %q: %w", sheets[0], err)
	}
	return rows, nil
}

func readCSVRows(path string) ([][]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no se pudo abrir el CSV: %w", err)
	}
	data = bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF")) // UTF-8 BOM added by Excel

	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = detectDelimiter(data)
	r.FieldsPerRecord = -1 // tolerate ragged rows; toTable pads them
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("CSV inválido: %w", err)
	}
	return rows, nil
}

// detectDelimiter picks ";" when the header line has more semicolons than
// commas. Good enough for spreadsheets exported by hand; it is not a
// general-purpose sniffer.
func detectDelimiter(data []byte) rune {
	firstLine, _, _ := bytes.Cut(data, []byte("\n"))
	if bytes.Count(firstLine, []byte(";")) > bytes.Count(firstLine, []byte(",")) {
		return ';'
	}
	return ','
}

// toTable trims cells, drops fully blank rows, pads short rows to the header
// width and names blank headers so every column is addressable.
func toTable(rows [][]string) (domain.Table, error) {
	if len(rows) == 0 || isBlank(rows[0]) {
		return domain.Table{}, fmt.Errorf("la primera fila debe contener los encabezados de columna")
	}

	headers := make([]string, len(rows[0]))
	seen := map[string]bool{}
	for i, h := range rows[0] {
		h = strings.TrimSpace(h)
		if h == "" {
			h = fmt.Sprintf("columna_%d", i+1)
		}
		key := domain.NormalizeKey(h)
		if seen[key] {
			return domain.Table{}, fmt.Errorf("la columna %q está repetida", h)
		}
		seen[key] = true
		headers[i] = h
	}

	var data [][]string
	for _, row := range rows[1:] {
		if isBlank(row) {
			continue
		}
		cells := make([]string, len(headers))
		for i := range cells {
			if i < len(row) {
				cells[i] = strings.TrimSpace(row[i])
			}
		}
		data = append(data, cells)
	}
	return domain.Table{Headers: headers, Rows: data}, nil
}

func isBlank(row []string) bool {
	for _, c := range row {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}

// ReadMessageTemplate looks for an optional "Mensaje" sheet (any case) with
// "asunto" and "cuerpo" columns (plus an optional "formato": html | texto)
// and returns its first data row as a starting
// template. found is false when the file has no such sheet — that is not an
// error, the user simply writes the message in the app.
func ReadMessageTemplate(path string) (tpl domain.Template, found bool, err error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".xlsx" && ext != ".xlsm" {
		return domain.Template{}, false, nil
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		return domain.Template{}, false, fmt.Errorf("no se pudo abrir el Excel: %w", err)
	}
	defer f.Close()

	for _, sheet := range f.GetSheetList() {
		if domain.NormalizeKey(sheet) != messageSheetName {
			continue
		}
		rows, err := f.GetRows(sheet)
		if err != nil || len(rows) < 2 {
			return domain.Template{}, false, nil
		}
		subjectCol, bodyCol, formatCol := -1, -1, -1
		for i, h := range rows[0] {
			switch domain.NormalizeKey(h) {
			case "asunto":
				subjectCol = i
			case "cuerpo", "mensaje":
				bodyCol = i
			case "formato":
				formatCol = i
			}
		}
		if subjectCol < 0 || bodyCol < 0 {
			return domain.Template{}, false, nil
		}
		body := cell(rows[1], bodyCol)
		// "formato" is optional: an explicit "html"/"texto" wins; without
		// it, a body laid out with block tags is clearly meant as HTML.
		isHTML := domain.IsMarkup(body)
		if formatCol >= 0 {
			switch strings.ToLower(cell(rows[1], formatCol)) {
			case "html":
				isHTML = true
			case "texto", "text", "plano":
				isHTML = false
			}
		}
		return domain.Template{
			Subject: cell(rows[1], subjectCol),
			Body:    body,
			HTML:    isHTML,
		}, true, nil
	}
	return domain.Template{}, false, nil
}

func cell(row []string, i int) string {
	if i < len(row) {
		return strings.TrimSpace(row[i])
	}
	return ""
}
