// Package application contains the use cases that orchestrate domain rules
// against infrastructure — this is where Excel, filesystem and (later)
// Outlook meet, but none of those details leak back into domain types.
package application

import (
	"fmt"
	"net/mail"
	"strings"

	"boleta-mailer/app/domain"
	"boleta-mailer/app/infrastructure/excel"
	"boleta-mailer/app/infrastructure/filesystem"
)

// ValidateBatch reads the spreadsheet at excelPath, resolves each employee's
// document inside folderPath, and returns a fully-validated Batch — every
// item already carries its final PENDING-stage status (READY or INVALID),
// so nothing sends until the caller explicitly starts the batch.
// context.md §15: validate the whole batch before any side effect.
func ValidateBatch(excelPath string, folderPath string) (domain.Batch, error) {
	workbook, err := excel.ReadWorkbook(excelPath)
	if err != nil {
		return domain.Batch{}, err
	}

	index, err := filesystem.BuildIndex(folderPath)
	if err != nil {
		return domain.Batch{}, fmt.Errorf("no se pudo leer la carpeta de documentos: %w", err)
	}

	seenDNI := make(map[string]bool, len(workbook.Employees))
	items := make([]domain.BatchItem, 0, len(workbook.Employees))

	for _, employee := range workbook.Employees {
		item := domain.BatchItem{Employee: employee, Status: domain.StatusReady}

		switch {
		case employee.DNI == "":
			item.Status = domain.StatusInvalid
			item.ValidationField = "dni"
			item.ValidationError = "DNI vacío"
		case seenDNI[employee.DNI]:
			item.Status = domain.StatusInvalid
			item.ValidationField = "dni"
			item.ValidationError = "DNI duplicado en la lista de destinatarios"
		case employee.Name == "":
			item.Status = domain.StatusInvalid
			item.ValidationField = "nombre"
			item.ValidationError = "Nombre vacío"
		case !isValidEmail(employee.Email):
			item.Status = domain.StatusInvalid
			item.ValidationField = "correo"
			item.ValidationError = "Formato de correo inválido"
		}
		seenDNI[employee.DNI] = true

		if item.Status != domain.StatusInvalid {
			path, resolveErr := index.Resolve(employee.DNI)
			if resolveErr != nil {
				item.Status = domain.StatusInvalid
				item.ValidationField = "archivo"
				item.ValidationError = resolveErr.Error()
			} else {
				item.Document = domain.Document{Path: path}
			}
		}

		items = append(items, item)
	}

	return domain.Batch{Items: items, Template: workbook.Template}, nil
}

func isValidEmail(email string) bool {
	if strings.TrimSpace(email) == "" {
		return false
	}
	_, err := mail.ParseAddress(email)
	return err == nil
}
