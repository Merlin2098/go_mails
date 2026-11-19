// Piezas visuales compartidas: íconos (SVG de trazo, sin dependencias) y
// la etiqueta de estado de cada correo.
import type { ItemStatus } from "../api";

type IconProps = { size?: number };
const svg = (size: number) => ({
  width: size,
  height: size,
  viewBox: "0 0 24 24",
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 2,
  strokeLinecap: "round" as const,
  strokeLinejoin: "round" as const,
  "aria-hidden": true,
});

export const Icon = {
  Send: ({ size = 16 }: IconProps) => (
    <svg {...svg(size)} strokeWidth={2.2}><path d="M22 2L11 13" /><path d="M22 2l-7 20-4-9-9-4z" /></svg>
  ),
  Check: ({ size = 12 }: IconProps) => <svg {...svg(size)} strokeWidth={3}><path d="M5 12l5 5 9-10" /></svg>,
  Arrow: ({ size = 16 }: IconProps) => <svg {...svg(size)}><path d="M5 12h14M13 6l6 6-6 6" /></svg>,
  Left: ({ size = 16 }: IconProps) => <svg {...svg(size)}><path d="M15 6l-6 6 6 6" /></svg>,
  Right: ({ size = 16 }: IconProps) => <svg {...svg(size)}><path d="M9 6l6 6-6 6" /></svg>,
  Mail: ({ size = 22 }: IconProps) => <svg {...svg(size)}><rect x="2" y="4" width="20" height="16" rx="2" /><path d="M2 7l10 6 10-6" /></svg>,
  Server: ({ size = 22 }: IconProps) => (
    <svg {...svg(size)}><rect x="3" y="4" width="18" height="7" rx="2" /><rect x="3" y="13" width="18" height="7" rx="2" /><path d="M7 7.5h.01M7 16.5h.01" /></svg>
  ),
  File: ({ size = 22 }: IconProps) => (
    <svg {...svg(size)}><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><path d="M14 2v6h6" /></svg>
  ),
  Doc: ({ size = 22 }: IconProps) => (
    <svg {...svg(size)}><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" /><path d="M14 2v6h6" /><path d="M9 15l2 2 4-4" /></svg>
  ),
  Clock: ({ size = 12 }: IconProps) => <svg {...svg(size)}><circle cx="12" cy="12" r="9" /><path d="M12 7v5l3 2" /></svg>,
  Monitor: ({ size = 12 }: IconProps) => <svg {...svg(size)}><rect x="3" y="4" width="18" height="12" rx="2" /><path d="M8 20h8" /></svg>,
  Download: ({ size = 16 }: IconProps) => <svg {...svg(size)}><path d="M12 3v12M7 10l5 5 5-5M5 21h14" /></svg>,
  Play: ({ size = 16 }: IconProps) => (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" aria-hidden><path d="M7 5v14l12-7z" /></svg>
  ),
};

const STATUS: Record<ItemStatus, { cls: string; label: string }> = {
  INVALID: { cls: "err", label: "Con error" },
  READY: { cls: "mute", label: "En cola" },
  SENDING: { cls: "info", label: "Enviando…" },
  SENT: { cls: "ok", label: "Enviado" },
  FAILED: { cls: "err", label: "Falló" },
  CANCELLED: { cls: "mute", label: "Cancelado" },
};

export function StatusPill({ status, readyLabel }: { status: ItemStatus; readyLabel?: string }) {
  const s = STATUS[status];
  return <span className={`pill ${s.cls}`}>{status === "READY" && readyLabel ? readyLabel : s.label}</span>;
}

export function Footer({ back, next, children }: { back?: () => void; next?: { label: string; onClick: () => void; disabled?: boolean }; children?: React.ReactNode }) {
  return (
    <div className="footer">
      <div className="group">
        {back && <button className="btn" onClick={back}>Atrás</button>}
        {children}
      </div>
      {next && (
        <button className="btn pri" onClick={next.onClick} disabled={next.disabled}>
          {next.label}
          <Icon.Arrow />
        </button>
      )}
    </div>
  );
}

// Insert {{variable}} chips: the same list everywhere variables are offered.
export function VariableChips({ headers, onPick }: { headers: string[]; onPick: (token: string) => void }) {
  return (
    <div className="row" style={{ gap: 6 }}>
      {headers.map((h) => (
        <button key={h} type="button" className="pill info mono" onClick={() => onPick(`{{${h}}}`)}>
          {`{{${h}}}`}
        </button>
      ))}
    </div>
  );
}
