import { useState } from "react";
import { api, errorText, Mapping } from "../api";
import { Footer, Icon, StatusPill } from "../components/ui";
import { useStore } from "../state/store";

const ROLES: { key: keyof Mapping; label: string; required?: boolean }[] = [
  { key: "email", label: "Correo destino", required: true },
  { key: "name", label: "Nombre visible" },
  { key: "key", label: "Clave de adjunto" },
  { key: "cc", label: "Copia (CC)" },
];

export default function DataStep() {
  const { state, dispatch } = useStore();
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const { table, validation } = state;

  async function pick() {
    const path = await api.selectDataFile();
    if (!path) return;
    setLoading(true);
    try {
      dispatch({ type: "tableLoaded", table: await api.loadData(path) });
      setError("");
    } catch (e) {
      setError(errorText(e));
    }
    setLoading(false);
  }

  const items = validation?.items ?? [];
  const emailErrors = items.filter((i) => i.status === "INVALID" && i.field !== "adjunto").length;

  return (
    <>
      <div className="content">
        <div className="head">
          <div>
            <h1>Destinatarios</h1>
            <p className="sub">Cualquier Excel o CSV. Tú eliges qué columna es el correo; el resto se vuelven variables.</p>
          </div>
          {table ? (
            <div className="filebox">
              <span style={{ color: "var(--ok)" }}><Icon.File size={20} /></span>
              <div style={{ fontSize: 13 }}>
                <div className="mono" style={{ fontWeight: 500 }}>{table.fileName}</div>
                <div className="muted">{table.rows?.length ?? 0} filas · {table.headers.length} columnas</div>
              </div>
              <button className="btn sm" onClick={pick} disabled={state.running}>Cambiar</button>
            </div>
          ) : (
            <button className="btn pri" onClick={pick} disabled={loading}>{loading ? "Leyendo…" : "Seleccionar Excel o CSV"}</button>
          )}
        </div>

        {error && <div className="banner err">{error}</div>}

        {!table && !error && (
          <div className="card muted">
            La primera fila debe tener los encabezados. Una columna con el correo es suficiente; cualquier otra
            (nombre, curso, fecha…) se podrá usar en el mensaje como <span className="mono">{"{{columna}}"}</span>.
          </div>
        )}

        {table && (
          <>
            <div className="card row" style={{ gap: "12px 28px" }}>
              <span className="label" style={{ width: "100%", margin: 0 }}>Mapeo de columnas</span>
              {ROLES.map((r) => (
                <label key={r.key} className="map">
                  {r.label}
                  <select value={state.mapping[r.key]} onChange={(e) => dispatch({ type: "mapping", patch: { [r.key]: e.target.value } })}>
                    {!r.required && <option value="">— ninguna —</option>}
                    {r.required && !state.mapping.email && <option value="">Elegir…</option>}
                    {table.headers.map((h) => <option key={h} value={h}>{h}</option>)}
                  </select>
                </label>
              ))}
            </div>

            <div className="row">
              <span className="pill mute">{table.rows?.length ?? 0} filas</span>
              {validation && !validation.error && (
                <>
                  <span className="pill ok">{items.length - emailErrors} con datos válidos</span>
                  {emailErrors > 0 && <span className="pill err">{emailErrors} con errores</span>}
                </>
              )}
              <span style={{ flex: 1 }} />
              <span className="muted" style={{ fontSize: 13 }}>Las filas con error no se envían</span>
            </div>
            {validation?.error && <div className="banner warn">{validation.error}</div>}

            <div className="card flush" style={{ flex: 1, minHeight: 240 }}>
              <div className="table-wrap" style={{ maxHeight: 420 }}>
                <table>
                  <thead>
                    <tr>
                      <th>#</th>
                      {table.headers.map((h) => <th key={h}>{h}</th>)}
                      <th>Estado</th>
                    </tr>
                  </thead>
                  <tbody>
                    {table.rows?.map((row, i) => {
                      const item = items[i];
                      const bad = item?.status === "INVALID" && item.field !== "adjunto";
                      return (
                        <tr key={i} className={bad ? "bad" : ""}>
                          <td className="muted">{i + 2}</td>
                          {row.map((c, j) => <td key={j} className={j === 0 ? "mono" : ""}>{c}</td>)}
                          <td>{item && (bad ? <span className="pill err" title={item.error}>{item.error}</span> : <StatusPill status="READY" readyLabel="Lista" />)}</td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </div>
          </>
        )}
      </div>
      <Footer
        back={() => dispatch({ type: "goto", step: "connection" })}
        next={{ label: "Siguiente: plantilla", onClick: () => dispatch({ type: "goto", step: "template" }), disabled: !table || !state.mapping.email }}
      />
    </>
  );
}
