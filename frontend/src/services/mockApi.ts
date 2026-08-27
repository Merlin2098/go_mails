// Implementación de prueba del contrato Go<->React descrito en
// docs/specs/ui_spec_mvp.md, usada mientras el módulo de renombre (Tab 1)
// sigue pendiente de elaboración — ver docs/specs/ui_spec_mvp.md §3 "Nota de
// implementación" para el estado real vs mock del resto de la app.

import type { RenamePreviewItem, RenameResult } from "../types/batch";

const delay = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

// ---- Tab Renombrar ----

export async function selectFolder(): Promise<string> {
  await delay(300);
  return "C:\\Boletas\\Agosto2026";
}

export async function previewRename(folderPath: string): Promise<RenamePreviewItem[]> {
  await delay(500);
  const mockFiles = ["BOLETA123.pdf", "BOLETA456.pdf", "BOLETA789.pdf"];
  const mockNewNames = ["12345678-Juan Perez.pdf", "87654321-Ana Lopez.pdf", "45678912-Luis Ramos.pdf"];
  return mockFiles.map((originalName, i) => ({
    originalName,
    newName: mockNewNames[i],
    fullPath: `${folderPath}\\${originalName}`,
  }));
}

export async function confirmRename(
  items: RenamePreviewItem[],
  onProgress: (done: number, total: number) => void
): Promise<RenameResult> {
  const total = items.length;
  for (let i = 0; i < total; i++) {
    await delay(200);
    onProgress(i + 1, total);
  }
  return { renamed: total, failed: 0, errors: [] };
}
