import { useEffect, useState } from "react";
import { api, AttachmentMode, RenamePlan, RenameStatus } from "../api";
import { Footer, VariableChips } from "../components/ui";
import { sendable, useStore } from "../state/store";

const RENAME_LABEL: Record<RenameStatus, { cls: string; label: string }> = {
  READY: { cls: "ok", label: "Listo" },
  SAME: { cls: "mute", label: "Sin cambios" },
  MISSING: { cls: "err", label: "No existe" },
  CONFLICT: { cls: "warn", label: "Conflicto" },
  INVALID: { cls: "err", label: "Inválido" },
  RENAMED: { cls: "ok", label: "Renombrado" },
  FAILED: { cls: "err", label: "Falló" },
};

export default function AttachmentsStep() {
  const { state, dispatch } = useStore();
  const { table, attachments: rule, validation, mapping } = state;
  const headers = table?.headers ?? [];
  const set = (patch: Partial<typeof rule>) => dispatch({ type: "attachments", patch });

  async function pickFolder() {
    const dir = await api.selectFolder("Carpeta de documentos");
    if (dir) set({ folder: dir, mode: rule.mode === "none" ? "pattern" : rule.mode });
  }

  // Sensible default the first time the user switches to "pattern".
  useEffect(() => {
    if (rule.mode === "pattern" && !rule.pattern && mapping.key) set({ pattern: `*{{${mapping.key}}}*` });
    if (rule.mode === "column" && !rule.column && headers.length) set({ column: headers[0] });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rule.mode]);

  const items = validation?.items ?? [];
  const ready = sendable(validation).length;
  const missing = items.filter((i) => i.status === "INVALID" && i.field === "adjunto");
  const otherInvalid = items.filter((i) => i.status === "INVALID" && i.field !== "adjunto");

  return (
    <>
      <div className="content">
        <div className="head">
          <div>
            <h1>Documentos</h1>
            <p className="sub">Renombra los archivos con datos del Excel y asocia cada uno a su destinatario.</p>
          </div>
        </div>

        <div className="row" style={{ alignItems: "flex-end" }}>
          <div style={{ flex: 1, minWidth: 320 }}>
            <label className="label" htmlFor="folder">Carpeta de documentos</label>
            <input id="folder" className="field mono" value={rule.folder} placeholder="Ninguna carpeta seleccionada" readOnly />
          </div>
          <button className="btn" onClick={pickFolder}>Elegir carpeta</button>
          <label className="check" style={{ height: 40 }}>
            <input type="checkbox" checked={rule.recursive} onChange={(e) => set({ recursive: e.target.checked })} />
            Incluir subcarpetas
          </label>
        </div>

        <div className="grid-2" style={{ gridTemplateColumns: "minmax(0,1.35fr) minmax(0,1fr)" }}>
          <RenameCard />

          <div className="card stack">
            <h2>2 · Asociar adjuntos</h2>
            <fieldset style={{ border: 0, padding: 0, margin: 0 }} className="stack">
              <legend className="label">Modo</legend>
              {([
                ["pattern", "Por patrón de nombre"],
                ["column", "Nombre o ruta en una columna"],
                ["none", "Sin adjunto (solo correo)"],
              ] as [AttachmentMode, string][]).map(([mode, label]) => (
                <label key={mode} className="check">
                  <input type="radio" name="mode" checked={rule.mode === mode} onChange={() => set({ mode })} />
                  {label}
                </label>
              ))}
            </fieldset>

            {rule.mode === "pattern" && (
              <div>
                <label className="label" htmlFor="pattern">Patrón (admite * y ?)</label>
                <input id="pattern" className="field mono" value={rule.pattern} placeholder="CERT-{{documento_id}}_*.pdf" onChange={(e) => set({ pattern: e.target.value })} />
                <div style={{ marginTop: 8 }}>
                  <VariableChips headers={headers} onPick={(t) => set({ pattern: rule.pattern + t })} />
                </div>
              </div>
            )}
            {rule.mode === "column" && (
              <div>
                <label className="label" htmlFor="col">Columna con el archivo (varios separados por ;)</label>
                <select id="col" className="field" value={rule.column} onChange={(e) => set({ column: e.target.value })}>
                  {headers.map((h) => <option key={h} value={h}>{h}</option>)}
                </select>
              </div>
            )}

            {validation?.attachmentError && <div className="banner err">{validation.attachmentError}</div>}

            {validation && !validation.error && (
              <div className="stack" style={{ gap: 8 }}>
                <div className="stat"><span>{rule.mode === "none" ? "Listos para enviar" : "Con adjunto listo"}</span><b style={{ color: "var(--ok)" }}>{ready}</b></div>
                {missing.length > 0 && (
                  <div className="stat">
                    <span>Archivo no encontrado<br /><span className="mono muted" style={{ fontSize: 12 }}>{missing.slice(0, 3).map((m) => m.name || m.email).join(", ")}{missing.length > 3 ? "…" : ""}</span></span>
                    <b style={{ color: "var(--err)" }}>{missing.length}</b>
                  </div>
                )}
                {otherInvalid.length > 0 && (
                  <div className="stat">
                    <span>Excluidas por datos inválidos<br /><span className="mono muted" style={{ fontSize: 12 }}>{otherInvalid.slice(0, 3).map((m) => m.name || m.email).join(", ")}{otherInvalid.length > 3 ? "…" : ""}</span></span>
                    <b style={{ color: "var(--warn)" }}>{otherInvalid.length}</b>
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
      </div>
      <Footer
        back={() => dispatch({ type: "goto", step: "template" })}
        next={{
          label: ready ? `Revisar y enviar ${ready} correos` : "Revisar y enviar",
          onClick: () => dispatch({ type: "goto", step: "send" }),
          disabled: !ready || !!validation?.attachmentError || !!validation?.templateError,
        }}
      />
    </>
  );
}

function RenameCard() {
  const { state, dispatch } = useStore();
  const { table, attachments: rule, mapping } = state;
  const headers = table?.headers ?? [];
  const [source, setSource] = useState(() => headers.find((h) => /origen|archivo|file/i.test(h)) ?? "");
  const [pattern, setPattern] = useState(() =>
    mapping.key ? `{{${mapping.key}}}${mapping.name ? `_{{${mapping.name}}}` : ""}{{ext}}` : ""
  );
  const [plan, setPlan] = useState<RenamePlan | null>(null);
  const [busy, setBusy] = useState(false);

  const req = { folder: rule.folder, sourceColumn: source, rule: pattern };

  useEffect(() => {
    if (!table || !rule.folder || !source) return setPlan(null);
    const t = setTimeout(() => api.previewRename(req).then(setPlan), 250);
    return () => clearTimeout(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [rule.folder, source, pattern, table]);

  async function run(action: "apply" | "undo") {
    setBusy(true);
    const result = action === "apply" ? await api.applyRename(req) : await api.undoRename();
    setPlan(result);
    setBusy(false);
    dispatch({ type: "revalidate" }); // the folder changed: re-check attachments
  }

  return (
    <div className="card stack">
      <div className="row">
        <h2>1 · Renombrar</h2>
        <span className="pill mute">opcional</span>
        <span style={{ flex: 1 }} />
        {plan?.canUndo && <button className="btn sm" onClick={() => run("undo")} disabled={busy}>Deshacer último</button>}
      </div>
      <div className="grid-2" style={{ gap: 12 }}>
        <div>
          <label className="label" htmlFor="src-col">Columna con el nombre actual</label>
          <select id="src-col" className="field" value={source} onChange={(e) => setSource(e.target.value)}>
            <option value="">— no renombrar —</option>
            {headers.map((h) => <option key={h} value={h}>{h}</option>)}
          </select>
        </div>
        <div>
          <label className="label" htmlFor="rule">Nuevo nombre</label>
          <input id="rule" className="field mono" value={pattern} placeholder="CERT-{{documento_id}}_{{nombre}}{{ext}}" onChange={(e) => setPattern(e.target.value)} />
        </div>
      </div>
      <p className="muted" style={{ margin: 0, fontSize: 12 }}>
        <span className="mono">{"{{ext}}"}</span> conserva la extensión original. Los caracteres no válidos en Windows se reemplazan por «_».
      </p>

      {!rule.folder && <div className="banner info">Elige la carpeta de documentos para ver la vista previa.</div>}
      {plan?.error && <div className="banner warn">{plan.error}</div>}
      {!!plan?.ops?.length && (
        <div className="table-wrap" style={{ maxHeight: 300, border: "1px solid var(--border-soft)", borderRadius: 8 }}>
          <table>
            <thead><tr><th>Archivo actual</th><th aria-hidden /><th>Nuevo nombre</th><th>Estado</th></tr></thead>
            <tbody>
              {plan.ops.map((op) => (
                <tr key={op.row}>
                  <td className="mono muted">{op.from || "—"}</td>
                  <td aria-hidden className="muted">→</td>
                  <td className="mono">{op.to || "—"}</td>
                  <td><span className={`pill ${RENAME_LABEL[op.status].cls}`} title={op.detail}>{RENAME_LABEL[op.status].label}</span></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      {plan && !plan.error && (
        <div className="row" style={{ justifyContent: "flex-end" }}>
          {plan.done > 0 && <span className="pill ok">{plan.done} renombrados</span>}
          <button className="btn pri sm" onClick={() => run("apply")} disabled={busy || plan.ready === 0}>
            Renombrar {plan.ready} archivos
          </button>
        </div>
      )}
    </div>
  );
}
