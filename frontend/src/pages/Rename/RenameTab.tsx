import { useState } from "react";
import * as api from "../../services/mockApi";
import type { RenamePreviewItem, RenameResult } from "../../types/batch";
import ProgressBar from "../../components/ProgressBar";

type Phase = "idle" | "previewing" | "ready" | "renaming" | "done";

export default function RenameTab() {
  const [folderPath, setFolderPath] = useState("");
  const [preview, setPreview] = useState<RenamePreviewItem[]>([]);
  const [phase, setPhase] = useState<Phase>("idle");
  const [progress, setProgress] = useState({ done: 0, total: 0 });
  const [result, setResult] = useState<RenameResult | null>(null);

  async function handleSelectFolder() {
    const path = await api.selectFolder();
    setFolderPath(path);
    setResult(null);
    setPhase("previewing");
    const items = await api.previewRename(path);
    setPreview(items);
    setPhase(items.length > 0 ? "ready" : "idle");
  }

  async function handleRename() {
    setPhase("renaming");
    setProgress({ done: 0, total: preview.length });
    const res = await api.confirmRename(preview, (done, total) => setProgress({ done, total }));
    setResult(res);
    setPhase("done");
  }

  const canRename = phase === "ready" && preview.length > 0;

  return (
    <div className="tab-panel">
      <section className="field-group">
        <label>Carpeta</label>
        <div className="field-row">
          <input type="text" readOnly value={folderPath} placeholder="Ninguna carpeta seleccionada" />
          <button onClick={handleSelectFolder} disabled={phase === "renaming"}>
            Seleccionar carpeta
          </button>
        </div>
      </section>

      {preview.length > 0 && (
        <section className="field-group">
          <label>Vista previa</label>
          <table className="preview-table">
            <thead>
              <tr>
                <th>Original</th>
                <th>Nuevo</th>
              </tr>
            </thead>
            <tbody>
              {preview.map((item) => (
                <tr key={item.fullPath}>
                  <td>{item.originalName}</td>
                  <td>{item.newName}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      )}

      {phase === "renaming" && (
        <section className="field-group">
          <ProgressBar processed={progress.done} total={progress.total} running={phase === "renaming"} />
        </section>
      )}

      {result && (
        <section className="field-group result-summary">
          <p>
            Renombrados: <strong>{result.renamed}</strong> · Errores: <strong>{result.failed}</strong>
          </p>
          {result.errors.length > 0 && (
            <ul className="error-list">
              {result.errors.map((err, i) => (
                <li key={i}>{err}</li>
              ))}
            </ul>
          )}
        </section>
      )}

      <div className="actions">
        <button className="btn-primary" onClick={handleRename} disabled={!canRename}>
          Renombrar
        </button>
      </div>
    </div>
  );
}
