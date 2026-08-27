// Cliente real (no mock) de los bindings Wails expuestos por app.go.
// Conforme cada pieza del backend se implementa de verdad, se mueve aquí
// desde services/mockApi.ts — ver docs/specs/ui_spec_mvp.md §3
// "Nota de implementación" para el estado actual de qué es real vs mock.
import { EventsOn } from "../../wailsjs/runtime/runtime";
import {
  CancelBatch,
  ExportReport,
  RetryFailed,
  SelectDocumentsFolder,
  SelectExcel,
  StartBatch,
  TestOutlookConnection,
  ValidateBatch,
} from "../../wailsjs/go/main/App";
import type { BatchProgress, BatchValidationSummary, OutlookStatus } from "../types/batch";

export async function testOutlookConnection(): Promise<OutlookStatus> {
  const result = await TestOutlookConnection();
  return {
    available: result.Available,
    email: result.Email,
    detail: result.Detail,
  };
}

export async function selectDocumentsFolder(): Promise<string> {
  return SelectDocumentsFolder();
}

export async function selectExcel(): Promise<string> {
  return SelectExcel();
}

export async function validateBatch(
  excelPath: string,
  folderPath: string
): Promise<BatchValidationSummary> {
  const result = await ValidateBatch(excelPath, folderPath);
  return {
    total: result.Total,
    valid: result.Valid,
    invalid: result.Invalid,
    errors: (result.Errors ?? []).map((e) => ({
      row: e.Row,
      field: e.Field,
      message: e.Message,
    })),
  };
}

// awaitBatchCompletion se suscribe a los eventos que runBatchWorker emite
// (ver app.go) y resuelve cuando el lote termina (processed === total) o
// rechaza si hubo un error fatal (Outlook no pudo conectar al iniciar el
// worker). Compartida por startBatch y retryFailed — ambas disparan el
// mismo worker en Go, solo cambia qué items procesa.
function awaitBatchCompletion(onProgress: (progress: BatchProgress) => void): Promise<void> {
  return new Promise((resolve, reject) => {
    const unsubscribeProgress = EventsOn(
      "batch:progress",
      (payload: { Total: number; Processed: number; Sent: number; Failed: number; Current: string }) => {
        const progress: BatchProgress = {
          batchId: "",
          total: payload.Total,
          processed: payload.Processed,
          sent: payload.Sent,
          failed: payload.Failed,
          current: payload.Current,
        };
        onProgress(progress);
        if (progress.processed >= progress.total) {
          unsubscribeProgress();
          unsubscribeFatal();
          resolve();
        }
      }
    );

    const unsubscribeFatal = EventsOn("batch:fatal-error", (message: string) => {
      unsubscribeProgress();
      unsubscribeFatal();
      reject(new Error(message));
    });
  });
}

// startBatch dispara el envío real y resuelve la promesa cuando el backend
// reporta que el lote terminó. onProgress se llama por cada evento
// intermedio.
export async function startBatch(onProgress: (progress: BatchProgress) => void): Promise<void> {
  await StartBatch();
  return awaitBatchCompletion(onProgress);
}

export async function cancelBatch(): Promise<void> {
  await CancelBatch();
}

// retryFailed reenvía únicamente los ítems que quedaron en FAILED tras el
// último StartBatch/RetryFailed (context.md §15) — nunca reenvía los ya
// exitosos. Misma mecánica de eventos que startBatch.
export async function retryFailed(onProgress: (progress: BatchProgress) => void): Promise<void> {
  await RetryFailed();
  return awaitBatchCompletion(onProgress);
}

// exportReport abre el diálogo nativo "Guardar como" en Go y escribe el
// reporte. Devuelve "" si el usuario cancela el diálogo.
export async function exportReport(): Promise<string> {
  return ExportReport();
}
