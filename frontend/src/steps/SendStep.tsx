import { useEffect, useRef, useState } from "react";
import { api, errorText } from "../api";
import { Footer, Icon, StatusPill } from "../components/ui";
import { sendable, useStore } from "../state/store";

export default function SendStep() {
  const { state, dispatch } = useStore();
  const { validation, running, logs, done, fatal, provider } = state;
  const [error, setError] = useState("");
  const [report, setReport] = useState("");
  const logRef = useRef<HTMLDivElement>(null);

  const items = sendable(validation);
  const count = (s: string) => items.filter((i) => i.status === s).length;
  const sent = count("SENT");
  const failed = count("FAILED");
  const cancelled = count("CANCELLED");
  const queued = count("READY");
  const processed = sent + failed + cancelled;
  const pct = items.length ? Math.round((processed / items.length) * 100) : 0;
  const delay = provider?.limits.delaySeconds ?? 0;
  const remaining = queued + count("SENDING");

  useEffect(() => {
    logRef.current?.scrollTo({ top: logRef.current.scrollHeight });
  }, [logs.length]);

  async function start(kind: "start" | "retry") {
    setError("");
    try {
      const started = kind === "start" ? await api.startBatch() : await api.retryFailed();
      if (started) {
        dispatch({ type: "runStarted" });
        // Retry resets FAILED/CANCELLED items to READY in Go; pull that state.
        dispatch({ type: "validated", result: await api.snapshot() });
      }
    } catch (e) {
      setError(errorText(e));
    }
  }

  async function exportReport() {
    try {
      const path = await api.exportReport();
      if (path) setReport(path);
    } catch (e) {
      setError(errorText(e));
    }
  }

  const heading = running ? "Enviando lote…" : done ? (done.cancelled ? "Lote cancelado" : "Lote terminado") : "Listo para enviar";

  return (
    <>
      <div className="content">
        <div className="head">
          <div>
            <h1>{heading}</h1>
            <p className="sub">Un worker en Go envía uno a uno en segundo plano; la interfaz nunca se congela.</p>
          </div>
          <div className="row" style={{ gap: 8 }}>
            {delay > 0 && <span className="pill info"><Icon.Clock />Pausa {delay} s</span>}
            {provider?.limits.keepAwake && <span className="pill mute"><Icon.Monitor />Equipo sin suspender</span>}
            {provider?.kind === "demo" && <span className="pill warn">Modo demo: solo .eml</span>}
          </div>
        </div>

        {fatal && <div className="banner err">No se pudo iniciar el envío: {fatal}</div>}
        {error && <div className="banner err">{error}</div>}
        {report && <div className="banner ok">Reporte guardado en <span className="mono">{report}</span></div>}
        {!items.length && <div className="banner warn">No hay correos listos. Revisa los pasos anteriores.</div>}

        <div className="card stack">
          <div className="kpis">
            <div className="kpi"><span>Procesados</span><b>{processed} / {items.length}</b></div>
            <div className="kpi"><span>Enviados</span><b style={{ color: "var(--ok)" }}>{sent}</b></div>
            <div className="kpi"><span>Fallidos</span><b style={{ color: "var(--err)" }}>{failed}</b></div>
            <div className="kpi"><span>Tiempo restante</span><b>{running ? `~${remaining * Math.max(delay, 1)} s` : "—"}</b></div>
          </div>
          <div className="bar" role="progressbar" aria-label="Progreso del envío" aria-valuemin={0} aria-valuemax={items.length} aria-valuenow={processed}>
            <i style={{ width: `${pct}%` }} />
          </div>
        </div>

        <div className="send-grid">
          <div className="card flush" style={{ overflow: "auto" }}>
            {items.map((it) => (
              <div key={it.index} className="item-row" title={it.error || undefined}>
                <span className="k">{it.key || it.row}</span>
                <span className="n">{it.name || "—"}</span>
                <span className="m">{it.email}</span>
                <StatusPill status={it.status} />
              </div>
            ))}
          </div>
          <div className="log" ref={logRef} aria-label="Registro del envío" aria-live="polite">
            {logs.length === 0 && <div className="t">El registro del worker aparecerá aquí.</div>}
            {logs.map((l, i) => (
              <div key={i}><span className="t">{l.time}</span> <span className={l.level}>{l.level.padEnd(5)}</span> {l.message}</div>
            ))}
          </div>
        </div>
      </div>
      <Footer back={running ? undefined : () => dispatch({ type: "goto", step: "attachments" })}>
        {running ? (
          <button className="btn danger" onClick={() => api.cancelBatch()}>Cancelar lote</button>
        ) : (
          <button className="btn pri" onClick={() => start("start")} disabled={queued === 0}>
            <Icon.Play />Enviar {queued} correos
          </button>
        )}
        <button className="btn" onClick={() => start("retry")} disabled={running || failed + cancelled === 0}>
          Reintentar fallidos ({failed + cancelled})
        </button>
        <button className="btn" onClick={exportReport} disabled={running || processed === 0}>
          <Icon.Download />Exportar reporte
        </button>
      </Footer>
    </>
  );
}
