// Command demodata writes the fictitious demo dataset (Excel + PDFs).
//
//	go run ./cmd/demodata -out demo -count 12
//	go run ./cmd/demodata -inbox tucorreo@gmail.com   # envío real, solo a tu bandeja
//
// Prefer scripts/demo-data.ps1 (or .sh), which also cleans the folder.
package main

import (
	"flag"
	"fmt"
	"log"

	"despacho/internal/demodata"
)

func main() {
	out := flag.String("out", "demo", "carpeta de salida")
	count := flag.Int("count", 12, "total de destinatarios (mínimo 12: incluye 2 errores sembrados)")
	inbox := flag.String("inbox", "", "opcional: tu correo; los destinatarios pasan a ser alias tucorreo+nombre@... (envío real a tu propia bandeja)")
	flag.Parse()
	if *count < 12 || *count > 5000 {
		log.Fatal("-count debe estar entre 12 y 5000")
	}

	res, err := demodata.Generate(*out, demodata.Options{Count: *count, Inbox: *inbox})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Excel: %s (%d destinatarios)\n", res.Workbook, res.Recipients)
	fmt.Printf("PDFs:  %s (%d archivos; %s omitido a propósito)\n", res.ScansDir, res.Scans, res.Missing)
}
