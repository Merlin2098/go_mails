// Tipos espejo de los structs Go definidos en app/domain y del contrato de
// bindings descrito en docs/specs/ui_spec_mvp.md. Mientras el backend real
// (Wails bindings) no exista, estos tipos son la fuente de verdad que tanto
// el mock (services/mockApi.ts) como el futuro cliente real deben cumplir.

export type BatchItemStatus =
  | "PENDING"
  | "VALIDATING"
  | "INVALID"
  | "READY"
  | "SENDING"
  | "SENT"
  | "FAILED"
  | "CANCELLED";

export interface RenamePreviewItem {
  originalName: string;
  newName: string;
  fullPath: string;
}

export interface RenameResult {
  renamed: number;
  failed: number;
  errors: string[];
}

export interface OutlookStatus {
  available: boolean;
  email: string;
  detail: string;
}

export interface ValidationError {
  row: number;
  field: string;
  message: string;
}

export interface BatchValidationSummary {
  total: number;
  valid: number;
  invalid: number;
  errors: ValidationError[];
}

export interface BatchProgress {
  batchId: string;
  total: number;
  processed: number;
  sent: number;
  failed: number;
  current: string;
}

export interface BatchItemResult {
  name: string;
  dni: string;
  email: string;
  file: string;
  status: BatchItemStatus;
  error?: string;
}
