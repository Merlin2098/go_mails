// Estado global del asistente. Un único useReducer en lugar de decenas de
// useState sueltos: cada transición tiene nombre y el estado nunca queda a
// medio actualizar. Las suscripciones a eventos de Go viven aquí (y no en la
// pantalla de envío) para que el progreso siga llegando aunque el usuario
// navegue a otro paso durante un lote.
import { createContext, ReactNode, useContext, useEffect, useMemo, useReducer, useRef } from "react";
import {
  api,
  AttachmentRule,
  CampaignRequest,
  DoneEvent,
  events,
  Item,
  LogLine,
  Mapping,
  ProviderState,
  TableData,
  TemplateDTO,
  ValidationResult,
} from "../api";

export type Step = "connection" | "data" | "template" | "attachments" | "send";
export const STEPS: { id: Step; label: string }[] = [
  { id: "connection", label: "Conexión" },
  { id: "data", label: "Datos" },
  { id: "template", label: "Plantilla" },
  { id: "attachments", label: "Adjuntos" },
  { id: "send", label: "Envío" },
];

export interface State {
  step: Step;
  provider: ProviderState | null;
  table: TableData | null;
  mapping: Mapping;
  template: TemplateDTO;
  attachments: AttachmentRule;
  validation: ValidationResult | null;
  revision: number; // forces re-validation, e.g. after files are renamed
  running: boolean;
  logs: LogLine[];
  done: DoneEvent | null;
  fatal: string;
}

type Action =
  | { type: "goto"; step: Step }
  | { type: "provider"; provider: ProviderState }
  | { type: "tableLoaded"; table: TableData }
  | { type: "mapping"; patch: Partial<Mapping> }
  | { type: "template"; patch: Partial<TemplateDTO> }
  | { type: "attachments"; patch: Partial<AttachmentRule> }
  | { type: "validated"; result: ValidationResult }
  | { type: "revalidate" }
  | { type: "runStarted" }
  | { type: "item"; item: Item }
  | { type: "log"; line: LogLine }
  | { type: "done"; done: DoneEvent }
  | { type: "fatal"; message: string };

const initial: State = {
  step: "connection",
  provider: null,
  table: null,
  mapping: { email: "", name: "", cc: "", key: "" },
  template: { subject: "", body: "", html: false },
  attachments: { mode: "none", folder: "", pattern: "", column: "", recursive: false },
  validation: null,
  revision: 0,
  running: false,
  logs: [],
  done: null,
  fatal: "",
};

function reducer(s: State, a: Action): State {
  switch (a.type) {
    case "goto":
      return { ...s, step: a.step };
    case "provider":
      return { ...s, provider: a.provider };
    case "tableLoaded": {
      // A template from the "Mensaje" sheet is only a starting point: it
      // never overwrites a message the user already wrote.
      const keepTemplate = s.template.subject || s.template.body;
      return {
        ...s,
        table: a.table,
        mapping: a.table.suggested,
        template: !keepTemplate && a.table.template ? a.table.template : s.template,
        validation: null,
        done: null,
      };
    }
    case "mapping":
      return { ...s, mapping: { ...s.mapping, ...a.patch } };
    case "template":
      return { ...s, template: { ...s.template, ...a.patch } };
    case "attachments":
      return { ...s, attachments: { ...s.attachments, ...a.patch } };
    case "validated":
      return { ...s, validation: a.result };
    case "revalidate":
      return { ...s, revision: s.revision + 1 };
    case "runStarted":
      return { ...s, running: true, done: null, fatal: "", logs: [] };
    case "item": {
      if (!s.validation?.items) return s;
      const items = s.validation.items.slice();
      items[a.item.index] = a.item;
      return { ...s, validation: { ...s.validation, items } };
    }
    case "log":
      return { ...s, logs: [...s.logs.slice(-199), a.line] };
    case "done":
      return { ...s, running: false, done: a.done };
    case "fatal":
      return { ...s, running: false, fatal: a.message };
  }
}

interface Ctx {
  state: State;
  dispatch: React.Dispatch<Action>;
  request: CampaignRequest;
}

const StoreContext = createContext<Ctx | null>(null);

export function StoreProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(reducer, initial);

  useEffect(() => {
    api.providerState().then((provider) => dispatch({ type: "provider", provider }));
    const offs = [
      events.onItem((e) => dispatch({ type: "item", item: e.item })),
      events.onLog((line) => dispatch({ type: "log", line })),
      events.onDone((done) => dispatch({ type: "done", done })),
      events.onFatal((message) => dispatch({ type: "fatal", message })),
    ];
    return () => offs.forEach((off) => off());
  }, []);

  const request = useMemo<CampaignRequest>(
    () => ({ mapping: state.mapping, template: state.template, attachments: state.attachments }),
    [state.mapping, state.template, state.attachments]
  );

  // Re-validate (debounced) whenever the campaign changes. Go is the single
  // source of truth for validation; the UI never re-implements the rules.
  // `running` is read through a ref on purpose: finishing a batch must not
  // trigger a re-validation by itself.
  const seq = useRef(0);
  const runningRef = useRef(state.running);
  runningRef.current = state.running;
  useEffect(() => {
    if (!state.table || runningRef.current) return;
    const id = ++seq.current;
    const timer = setTimeout(async () => {
      const result = await api.validate(request);
      if (id === seq.current) dispatch({ type: "validated", result });
    }, 250);
    return () => clearTimeout(timer);
  }, [request, state.table, state.revision]);

  return <StoreContext.Provider value={{ state, dispatch, request }}>{children}</StoreContext.Provider>;
}

export function useStore(): Ctx {
  const ctx = useContext(StoreContext);
  if (!ctx) throw new Error("useStore fuera de StoreProvider");
  return ctx;
}

// Derived helpers shared by several screens.
export function sendable(v: ValidationResult | null): Item[] {
  return (v?.items ?? []).filter((i) => i.status !== "INVALID");
}

export function stepDone(s: State, step: Step): boolean {
  switch (step) {
    case "connection":
      return !!s.provider?.kind;
    case "data":
      return !!s.table && !!s.mapping.email;
    case "template":
      return !!s.template.subject.trim() && !!s.validation && !s.validation.templateError;
    case "attachments":
      return !!s.validation && !s.validation.error && !s.validation.attachmentError && sendable(s.validation).length > 0;
    case "send":
      return !!s.done && s.done.failed === 0 && s.done.cancelled === 0;
  }
}
