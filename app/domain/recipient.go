package domain

import "strings"

// Table is the raw input dataset: one header row plus data rows, exactly as
// read from an .xlsx or .csv file. It is deliberately dumb — no column has a
// meaning yet. ColumnMapping (application layer) is what decides which
// column is the email, which is the name, and so on.
type Table struct {
	Headers []string
	Rows    [][]string
}

// Recipient is one row of the input table after column mapping. Fields holds
// every column of the row keyed by NormalizeKey(header), so any column the
// user adds to the spreadsheet is automatically usable as a {{variable}}.
type Recipient struct {
	Row    int // 1-based spreadsheet row (the header is row 1)
	Email  string
	CC     string
	Name   string
	Key    string
	Fields map[string]string
}

// NormalizeKey is the single rule for matching column headers and template
// variables: case-insensitive and ignoring surrounding spaces. "Nombre",
// " NOMBRE " and "nombre" all refer to the same column. Having one function
// for this avoids the classic bug where the reader and the renderer disagree
// on how keys are compared.
func NormalizeKey(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
