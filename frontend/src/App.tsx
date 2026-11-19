import { Icon } from "./components/ui";
import { STEPS, stepDone, StoreProvider, useStore } from "./state/store";
import ConnectionStep from "./steps/ConnectionStep";
import DataStep from "./steps/DataStep";
import TemplateStep from "./steps/TemplateStep";
import AttachmentsStep from "./steps/AttachmentsStep";
import SendStep from "./steps/SendStep";

const PROVIDER_LABEL: Record<string, string> = {
  outlook: "Outlook Desktop",
  smtp: "SMTP",
  demo: "Modo demo",
};

function Sidebar() {
  const { state, dispatch } = useStore();
  const p = state.provider;
  return (
    <nav className="side" aria-label="Pasos">
      <div className="brand">
        <span className="brand-mark"><Icon.Send /></span>
        Despacho
      </div>
      {STEPS.map((s, i) => {
        const on = state.step === s.id;
        const done = !on && stepDone(state, s.id);
        return (
          <button
            key={s.id}
            className={`step${on ? " on" : ""}${done ? " done" : ""}`}
            aria-current={on ? "step" : undefined}
            onClick={() => dispatch({ type: "goto", step: s.id })}
          >
            <span className="n">{done ? <Icon.Check /> : i + 1}</span>
            {s.label}
          </button>
        );
      })}
      <div className="side-status">
        {p?.kind ? (
          <>
            <strong>{PROVIDER_LABEL[p.kind]}</strong>
            <span>{p.kind === "demo" ? "Sin envíos reales" : p.account}</span>
          </>
        ) : (
          <>
            <strong>Sin cuenta conectada</strong>
            <span>Elige un proveedor para continuar</span>
          </>
        )}
      </div>
      <div className="side-foot">Go · Wails · React</div>
    </nav>
  );
}

function CurrentStep() {
  const { state } = useStore();
  switch (state.step) {
    case "connection":
      return <ConnectionStep />;
    case "data":
      return <DataStep />;
    case "template":
      return <TemplateStep />;
    case "attachments":
      return <AttachmentsStep />;
    case "send":
      return <SendStep />;
  }
}

export default function App() {
  return (
    <StoreProvider>
      <div className="shell">
        <Sidebar />
        <div className="main">
          <CurrentStep />
        </div>
      </div>
    </StoreProvider>
  );
}
