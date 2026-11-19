import { useEffect, useRef, useState } from "react";
import { api, MessagePreview } from "../api";
import { Footer, Icon, VariableChips } from "../components/ui";
import { useStore } from "../state/store";

type Target = "subject" | "body";

export default function TemplateStep() {
  const { state, dispatch, request } = useStore();
  const { table, template, validation } = state;
  const rows = table?.rows?.length ?? 0;
  const [index, setIndex] = useState(0);
  const [preview, setPreview] = useState<MessagePreview | null>(null);
  const subjectRef = useRef<HTMLInputElement>(null);
  const bodyRef = useRef<HTMLTextAreaElement>(null);
  const lastFocus = useRef<Target>("body");

  // Go renders the preview with the exact code path used for sending, so
  // what you see here is byte-for-byte what the recipient gets.
  useEffect(() => {
    if (!rows) return;
    const t = setTimeout(() => api.previewMessage(request, index).then(setPreview), 200);
    return () => clearTimeout(t);
  }, [request, index, rows]);

  function insert(token: string) {
    const target = lastFocus.current;
    const el = target === "subject" ? subjectRef.current : bodyRef.current;
    const value = template[target];
    const start = el?.selectionStart ?? value.length;
    const end = el?.selectionEnd ?? value.length;
    dispatch({ type: "template", patch: { [target]: value.slice(0, start) + token + value.slice(end) } });
    requestAnimationFrame(() => {
      el?.focus();
      el?.setSelectionRange(start + token.length, start + token.length);
    });
  }

  return (
    <>
      <div className="content">
        <div className="head">
          <div>
            <h1>Mensaje</h1>
            <p className="sub">Escribe una vez; cada correo se personaliza con las columnas del archivo.</p>
          </div>
          <div className="seg" role="group" aria-label="Formato del cuerpo">
            <button aria-pressed={!template.html} onClick={() => dispatch({ type: "template", patch: { html: false } })}>Texto plano</button>
            <button aria-pressed={template.html} onClick={() => dispatch({ type: "template", patch: { html: true } })}>HTML</button>
          </div>
        </div>

        {!table && <div className="banner warn">Carga primero el archivo de destinatarios para usar sus columnas como variables.</div>}
        {validation?.templateError && <div className="banner err">{validation.templateError}</div>}

        <div className="grid-2" style={{ flex: 1, minHeight: 420 }}>
          <div className="card stack">
            <div>
              <label className="label" htmlFor="subj">Asunto</label>
              <input id="subj" ref={subjectRef} className="field mono" value={template.subject} placeholder="Tu certificado — {{curso}}"
                onFocus={() => (lastFocus.current = "subject")}
                onChange={(e) => dispatch({ type: "template", patch: { subject: e.target.value } })} />
            </div>
            <div style={{ flex: 1, display: "flex", flexDirection: "column" }}>
              <label className="label" htmlFor="body">Cuerpo</label>
              <textarea id="body" ref={bodyRef} className="field mono" style={{ flex: 1, minHeight: 220 }} value={template.body}
                placeholder={"Hola {{nombre}},\n\n…"}
                onFocus={() => (lastFocus.current = "body")}
                onChange={(e) => dispatch({ type: "template", patch: { body: e.target.value } })} />
            </div>
            {table && (
              <div>
                <span className="label">Insertar variable</span>
                <VariableChips headers={table.headers} onPick={insert} />
              </div>
            )}
          </div>

          <div className="card flush" style={{ display: "flex", flexDirection: "column" }}>
            <div className="preview-head">
              <strong style={{ fontSize: 13 }}>Vista previa</strong>
              <span className="muted" style={{ fontSize: 13 }}>· fila {rows ? index + 2 : "—"} de {rows + 1}</span>
              <span style={{ flex: 1 }} />
              <button className="btn icon" aria-label="Destinatario anterior" disabled={!rows} onClick={() => setIndex((index - 1 + rows) % rows)}><Icon.Left /></button>
              <button className="btn icon" aria-label="Destinatario siguiente" disabled={!rows} onClick={() => setIndex((index + 1) % rows)}><Icon.Right /></button>
            </div>
            {preview && (
              <>
                <div className="envelope">
                  {preview.from && <div><span>De</span>{preview.from}</div>}
                  <div><span>Para</span><span className="mono">{preview.to || "—"}</span></div>
                  {preview.cc && <div><span>CC</span><span className="mono">{preview.cc}</span></div>}
                  <div><span>Asunto</span><strong>{preview.subject || "—"}</strong></div>
                </div>
                {preview.error && <div className="banner warn" style={{ margin: "12px 20px 0" }}>Esta fila no se enviará: {preview.error}</div>}
                {preview.html ? (
                  // sandbox="" : el HTML del usuario se muestra sin ejecutar scripts.
                  <iframe className="mail-frame" sandbox="" title="Vista previa HTML" srcDoc={preview.body} />
                ) : (
                  <div className="mail-body">{preview.body}</div>
                )}
                {!!preview.attachments?.length && (
                  <div className="row" style={{ padding: "14px 20px", borderTop: "1px solid var(--border-soft)", gap: 8 }}>
                    {preview.attachments.map((a) => (
                      <span key={a} className="attach"><span style={{ color: "#b42318" }}><Icon.File size={16} /></span>{a}</span>
                    ))}
                  </div>
                )}
              </>
            )}
          </div>
        </div>
      </div>
      <Footer
        back={() => dispatch({ type: "goto", step: "data" })}
        next={{ label: "Siguiente: adjuntos", onClick: () => dispatch({ type: "goto", step: "attachments" }) }}
      />
    </>
  );
}
