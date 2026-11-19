import { useEffect, useState } from "react";
import { api, errorText, Limits, OutlookStatus, ProviderKind, SMTPPreset, SMTPSettings } from "../api";
import { Footer, Icon } from "../components/ui";
import { useStore } from "../state/store";

type Choice = Exclude<ProviderKind, "">;

export default function ConnectionStep() {
  const { state, dispatch } = useStore();
  const [choice, setChoice] = useState<Choice>((state.provider?.kind || "outlook") as Choice);
  const connected = !!state.provider?.kind;

  return (
    <>
      <div className="content">
        <div>
          <h1>¿Desde dónde quieres enviar?</h1>
          <p className="sub">Los documentos salen desde tu propia cuenta. Nada pasa por servidores de terceros.</p>
        </div>

        <div className="grid-3">
          <ProviderCard
            choice="outlook" current={choice} onPick={setChoice}
            icon={<Icon.Mail />} tint="#e3f0fb" color="#0b5cad"
            title="Outlook Desktop" subtitle="Outlook Classic vía COM"
            text="Usa las cuentas ya configuradas en tu Outlook de escritorio. Sin contraseñas."
          />
          <ProviderCard
            choice="smtp" current={choice} onPick={setChoice}
            icon={<Icon.Server />} tint="#fdecea" color="#b42318"
            title="Gmail / SMTP" subtitle="Contraseña de aplicación"
            text="Gmail, Zoho o el servidor de tu hosting. Sin proyecto en Google Cloud."
          />
          <ProviderCard
            choice="demo" current={choice} onPick={setChoice}
            icon={<Icon.Doc />} tint="#eef0f2" color="#4a505b"
            title="Modo demo" subtitle="Genera archivos .eml"
            text="No envía nada: guarda cada correo para revisarlo antes de un envío real."
          />
        </div>

        <div className="card">
          {choice === "outlook" && <OutlookPanel />}
          {choice === "smtp" && <SMTPPanel />}
          {choice === "demo" && <DemoPanel />}
        </div>

        {state.provider && <LimitsCard limits={state.provider.limits} dailyLimit={state.provider.dailyLimit} />}
        {connected && <TestSend />}
      </div>
      <Footer next={{ label: "Siguiente: cargar datos", onClick: () => dispatch({ type: "goto", step: "data" }), disabled: !connected }} />
    </>
  );
}

function ProviderCard(props: {
  choice: Choice; current: Choice; onPick: (c: Choice) => void;
  icon: React.ReactNode; tint: string; color: string; title: string; subtitle: string; text: string;
}) {
  const { state } = useStore();
  const active = state.provider?.kind === props.choice;
  return (
    <button className="prov" aria-pressed={props.current === props.choice} onClick={() => props.onPick(props.choice)}>
      <div className="prov-top">
        <span className="prov-ico" style={{ background: props.tint, color: props.color }}>{props.icon}</span>
        <div style={{ flex: 1 }}>
          <div className="prov-title">{props.title}</div>
          <div className="prov-sub">{props.subtitle}</div>
        </div>
        {active && <span className="pill ok"><Icon.Check />Activa</span>}
      </div>
      <p>{props.text}</p>
    </button>
  );
}

function OutlookPanel() {
  const { state, dispatch } = useStore();
  const [status, setStatus] = useState<OutlookStatus | null>(null);
  const [checking, setChecking] = useState(false);
  const [account, setAccount] = useState(state.provider?.kind === "outlook" ? state.provider.account : "");
  const [error, setError] = useState("");

  async function check() {
    setChecking(true);
    setError("");
    const s = await api.checkOutlook();
    setStatus(s);
    if (!account && s.accounts?.length) setAccount(s.accounts[0].smtpAddress);
    setChecking(false);
  }

  async function use() {
    try {
      dispatch({ type: "provider", provider: await api.useOutlook(account) });
    } catch (e) {
      setError(errorText(e));
    }
  }

  return (
    <div className="stack">
      <div className="row">
        <button className="btn" onClick={check} disabled={checking}>{checking ? "Buscando Outlook…" : "Detectar Outlook"}</button>
        {status && <span className={`pill ${status.available ? "ok" : "err"}`}>{status.available ? `Outlook detectado · ${status.accounts?.length ?? 0} cuentas` : "No disponible"}</span>}
      </div>
      {status && !status.available && <div className="banner err">{status.detail}</div>}
      {status?.available && (
        <div className="row" style={{ alignItems: "flex-end" }}>
          <div style={{ flex: 1, minWidth: 260 }}>
            <label className="label" htmlFor="ol-acc">Cuenta de envío</label>
            <select id="ol-acc" className="field" value={account} onChange={(e) => setAccount(e.target.value)}>
              {status.accounts?.map((a) => (
                <option key={a.smtpAddress} value={a.smtpAddress}>{a.smtpAddress}{a.displayName !== a.smtpAddress ? ` — ${a.displayName}` : ""}</option>
              ))}
            </select>
          </div>
          <button className="btn pri" onClick={use} disabled={!account}>Usar esta cuenta</button>
        </div>
      )}
      {error && <div className="banner err">{error}</div>}
    </div>
  );
}

function SMTPPanel() {
  const { dispatch } = useStore();
  const [presets, setPresets] = useState<SMTPPreset[]>([]);
  const [s, setS] = useState<SMTPSettings>({ preset: "gmail", host: "smtp.gmail.com", port: 587, security: "starttls", username: "", password: "", fromName: "" });
  const [result, setResult] = useState<{ ok: boolean; detail: string } | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    api.smtpPresets().then(setPresets);
  }, []);

  const preset = presets.find((p) => p.id === s.preset);
  const set = (patch: Partial<SMTPSettings>) => {
    setS((prev) => ({ ...prev, ...patch }));
    setResult(null);
  };

  function pickPreset(id: string) {
    const p = presets.find((x) => x.id === id);
    if (p) set({ preset: id, host: p.host, port: p.port, security: p.security });
  }

  async function test() {
    setBusy(true);
    setResult(await api.testSMTP(s));
    setBusy(false);
  }

  async function use() {
    try {
      dispatch({ type: "provider", provider: await api.useSMTP(s) });
      setResult({ ok: true, detail: "Cuenta conectada." });
    } catch (e) {
      setResult({ ok: false, detail: errorText(e) });
    }
  }

  return (
    <div className="stack">
      <div className="grid-3">
        <div>
          <label className="label" htmlFor="smtp-preset">Proveedor</label>
          <select id="smtp-preset" className="field" value={s.preset} onChange={(e) => pickPreset(e.target.value)}>
            {presets.map((p) => <option key={p.id} value={p.id}>{p.label}</option>)}
          </select>
        </div>
        <div>
          <label className="label" htmlFor="smtp-host">Servidor</label>
          <input id="smtp-host" className="field mono" value={s.host} onChange={(e) => set({ host: e.target.value })} />
        </div>
        <div className="row" style={{ gap: 8, flexWrap: "nowrap" }}>
          <div style={{ width: 90 }}>
            <label className="label" htmlFor="smtp-port">Puerto</label>
            <input id="smtp-port" className="field mono" type="number" value={s.port} onChange={(e) => set({ port: Number(e.target.value) })} />
          </div>
          <div style={{ flex: 1 }}>
            <label className="label" htmlFor="smtp-sec">Seguridad</label>
            <select id="smtp-sec" className="field" value={s.security} onChange={(e) => set({ security: e.target.value })}>
              <option value="starttls">STARTTLS</option>
              <option value="tls">TLS</option>
            </select>
          </div>
        </div>
        <div>
          <label className="label" htmlFor="smtp-user">Correo</label>
          <input id="smtp-user" className="field" type="email" autoComplete="off" value={s.username} onChange={(e) => set({ username: e.target.value })} placeholder="tucorreo@gmail.com" />
        </div>
        <div>
          <label className="label" htmlFor="smtp-pass">Contraseña de aplicación</label>
          <input id="smtp-pass" className="field mono" type="password" autoComplete="off" value={s.password} onChange={(e) => set({ password: e.target.value })} />
        </div>
        <div>
          <label className="label" htmlFor="smtp-name">Nombre del remitente</label>
          <input id="smtp-name" className="field" value={s.fromName} onChange={(e) => set({ fromName: e.target.value })} placeholder="Opcional" />
        </div>
      </div>
      {preset && (
        <div className="row muted" style={{ fontSize: 13 }}>
          {preset.dailyLimit > 0 && <span className="pill warn">Límite del proveedor: {preset.dailyLimit} / día</span>}
          <span>{preset.help}</span>
          {preset.helpUrl && <button className="link" onClick={() => api.openURL(preset.helpUrl)}>Crear contraseña de aplicación</button>}
        </div>
      )}
      <div className="row">
        <button className="btn" onClick={test} disabled={busy}>{busy ? "Probando…" : "Probar conexión"}</button>
        <button className="btn pri" onClick={use} disabled={!s.username || !s.password}>Usar esta cuenta</button>
        <span className="muted" style={{ fontSize: 12 }}>La contraseña solo se guarda en memoria mientras la app está abierta.</span>
      </div>
      {result && <div className={`banner ${result.ok ? "ok" : "err"}`}>{result.detail}</div>}
    </div>
  );
}

function DemoPanel() {
  const { state, dispatch } = useStore();
  const [dir, setDir] = useState(state.provider?.kind === "demo" ? state.provider.account : "");

  async function pick() {
    const d = await api.selectFolder("Carpeta para los archivos .eml");
    if (d) setDir(d);
  }

  return (
    <div className="row" style={{ alignItems: "flex-end" }}>
      <div style={{ flex: 1, minWidth: 280 }}>
        <label className="label" htmlFor="demo-dir">Carpeta de salida</label>
        <input id="demo-dir" className="field mono" value={dir} placeholder="Documentos\Despacho\salida_demo" readOnly />
      </div>
      <button className="btn" onClick={pick}>Elegir carpeta</button>
      <button className="btn pri" onClick={async () => dispatch({ type: "provider", provider: await api.useDemo(dir) })}>Usar modo demo</button>
    </div>
  );
}

function LimitsCard({ limits, dailyLimit }: { limits: Limits; dailyLimit: number }) {
  const { dispatch } = useStore();
  const [draft, setDraft] = useState(limits);
  const [error, setError] = useState("");

  async function save(next: Limits) {
    setDraft(next);
    try {
      dispatch({ type: "provider", provider: await api.setLimits(next) });
      setError("");
    } catch (e) {
      setError(errorText(e));
    }
  }

  return (
    <div className="card stack">
      <div className="grid-3" style={{ alignItems: "end" }}>
        <div>
          <label className="label" htmlFor="delay">Pausa entre correos (segundos)</label>
          <input id="delay" className="field" type="number" min={0} max={120} value={draft.delaySeconds}
            onChange={(e) => setDraft({ ...draft, delaySeconds: Number(e.target.value) })} onBlur={() => save(draft)} />
        </div>
        <div>
          <label className="label" htmlFor="cap">Tope por lote</label>
          <input id="cap" className="field" type="number" min={1} value={draft.maxPerBatch}
            onChange={(e) => setDraft({ ...draft, maxPerBatch: Number(e.target.value) })} onBlur={() => save(draft)} />
        </div>
        <label className="check" style={{ height: 40 }}>
          <input type="checkbox" checked={draft.keepAwake} onChange={(e) => save({ ...draft, keepAwake: e.target.checked })} />
          <span><strong style={{ fontWeight: 500 }}>Evitar suspensión del equipo</strong><br /><span className="muted" style={{ fontSize: 12 }}>Mientras haya un lote en curso</span></span>
        </label>
      </div>
      {dailyLimit > 0 && draft.maxPerBatch > dailyLimit && (
        <div className="banner warn">Tu proveedor limita a {dailyLimit} destinatarios por día: un lote mayor quedará a medias.</div>
      )}
      {error && <div className="banner err">{error}</div>}
    </div>
  );
}

function TestSend() {
  const [to, setTo] = useState("");
  const [msg, setMsg] = useState<{ ok: boolean; text: string } | null>(null);
  const [busy, setBusy] = useState(false);

  async function send() {
    setBusy(true);
    try {
      await api.sendTest(to);
      setMsg({ ok: true, text: `Correo de prueba enviado a ${to}.` });
    } catch (e) {
      setMsg({ ok: false, text: errorText(e) });
    }
    setBusy(false);
  }

  return (
    <div className="row">
      <input className="field" style={{ maxWidth: 320 }} type="email" placeholder="correo@para-la-prueba.com" aria-label="Correo para la prueba" value={to} onChange={(e) => setTo(e.target.value)} />
      <button className="btn" onClick={send} disabled={!to || busy}>{busy ? "Enviando…" : "Enviar correo de prueba"}</button>
      {msg && <span className={`pill ${msg.ok ? "ok" : "err"}`}>{msg.text}</span>}
    </div>
  );
}
