#!/usr/bin/env bash
# Genera el set de datos ficticio para demos (plantilla Excel + PDFs).
# Equivalente a scripts/demo-data.ps1, para Git Bash / macOS / Linux.
#
#   scripts/demo-data.sh                 # 12 destinatarios en ./demo
#   scripts/demo-data.sh -n 100          # 100 destinatarios
#   scripts/demo-data.sh -o /tmp/x -k    # otra carpeta, sin limpiar antes
set -euo pipefail

out="demo"
count=12
keep=0
while getopts "o:n:kh" opt; do
  case "$opt" in
    o) out="$OPTARG" ;;
    n) count="$OPTARG" ;;
    k) keep=1 ;;
    h|*) sed -n '2,8p' "$0"; exit 0 ;;
  esac
done

root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$root"
command -v go >/dev/null || { echo "No se encontró 'go' en el PATH (https://go.dev/dl/)" >&2; exit 1; }

if [[ -d "$out" && "$keep" -eq 0 ]]; then
  echo "Limpiando $out ..."
  rm -rf -- "$out"
fi

go run ./cmd/demodata -out "$out" -count "$count"

cat <<EOF

Listo. En la app:
  1. Conexión  -> Modo demo
  2. Datos     -> $out/plantilla_envio.xlsx
  4. Adjuntos  -> carpeta $out/documentos_origen
                  renombrar con  CERT-{{documento_id}}_{{nombre}}{{ext}}
                  patrón         CERT-{{documento_id}}_*.pdf
EOF
