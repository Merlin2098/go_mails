// Única puerta de entrada al backend Go. Los bindings de wailsjs/ se generan
// a partir de los structs de Go (`wails generate module`); aquí se declaran
// interfaces planas equivalentes para que el resto de la UI no dependa de
// las clases generadas.
import { desktop } from "../wailsjs/go/models";
import * as Shell from "../wailsjs/go/desktop/App";
import * as Campaign from "../wailsjs/go/desktop/CampaignService";
import * as Providers from "../wailsjs/go/desktop/ProviderService";
import * as Rename from "../wailsjs/go/desktop/RenameService";
import { EventsOn } from "../wailsjs/runtime/runtime";

export type ProviderKind = "" | "outlook" | "smtp" | "demo";

export interface Limits {
  delaySeconds: number;
  maxPerBatch: number;
  keepAwake: boolean;
}

export interface ProviderState {
  kind: ProviderKind;
  account: string;
  dailyLimit: number;
  limits: Limits;
}

export interface OutlookStatus {
  available: boolean;
  detail: string;
  accounts: { displayName: string; smtpAddress: string }[] | null;
}

export interface SMTPSettings {
  preset: string;
  host: string;
  port: number;
  security: string;
  username: string;
  password: string;
  fromName: string;
}

export interface SMTPPreset {
  id: string;
  label: string;
  host: string;
  port: number;
  security: string;
  dailyLimit: number;
  helpUrl: string;
  help: string;
}

export interface Mapping {
  email: string;
  name: string;
  cc: string;
  key: string;
}

export interface TemplateDTO {
  subject: string;
  body: string;
  html: boolean;
}

export type AttachmentMode = "none" | "pattern" | "column";

export interface AttachmentRule {
  mode: AttachmentMode;
  folder: string;
  pattern: string;
  column: string;
  recursive: boolean;
}

export interface CampaignRequest {
  mapping: Mapping;
  template: TemplateDTO;
  attachments: AttachmentRule;
}

export interface TableData {
  path: string;
  fileName: string;
  headers: string[];
  rows: string[][];
  suggested: Mapping;
  template: TemplateDTO | null;
}

export type ItemStatus = "INVALID" | "READY" | "SENDING" | "SENT" | "FAILED" | "CANCELLED";

export interface Item {
  index: number;
  row: number;
  email: string;
  name: string;
  key: string;
  attachments: string[] | null;
  status: ItemStatus;
  field: string;
  error: string;
}

export interface ValidationResult {
  error: string; // tabla / mapeo
  templateError: string;
  attachmentError: string;
  total: number;
  ready: number;
  invalid: number;
  sent: number;
  failed: number;
  items: Item[] | null;
  running: boolean;
}

export interface MessagePreview {
  from: string;
  to: string;
  cc: string;
  subject: string;
  body: string;
  html: boolean;
  attachments: string[] | null;
  error: string;
}

export interface RenameRequest {
  folder: string;
  sourceColumn: string;
  rule: string;
}

export type RenameStatus = "READY" | "SAME" | "MISSING" | "CONFLICT" | "INVALID" | "RENAMED" | "FAILED";

export interface RenamePlan {
  error: string;
  ops: { row: number; from: string; to: string; status: RenameStatus; detail: string }[] | null;
  ready: number;
  done: number;
  canUndo: boolean;
}

export interface ItemEvent {
  item: Item;
  total: number;
  processed: number;
  sent: number;
  failed: number;
}

export interface LogLine {
  time: string;
  level: "INFO" | "WARN" | "ERROR";
  message: string;
}

export interface DoneEvent {
  sent: number;
  failed: number;
  cancelled: number;
}

// Los bindings devuelven JSON plano; el cast solo alinea los nombres de tipo.
const as = <T>(p: Promise<unknown>) => p as Promise<T>;

export const api = {
  selectDataFile: () => Shell.SelectDataFile(),
  selectFolder: (title: string) => Shell.SelectFolder(title),
  openURL: (url: string) => Shell.OpenURL(url),

  providerState: () => as<ProviderState>(Providers.State()),
  checkOutlook: () => as<OutlookStatus>(Providers.CheckOutlook()),
  useOutlook: (account: string) => as<ProviderState>(Providers.UseOutlook(account)),
  smtpPresets: () => as<SMTPPreset[]>(Providers.SMTPPresets()),
  testSMTP: (s: SMTPSettings) => as<{ ok: boolean; detail: string }>(Providers.TestSMTP(desktop.SMTPSettings.createFrom(s))),
  useSMTP: (s: SMTPSettings) => as<ProviderState>(Providers.UseSMTP(desktop.SMTPSettings.createFrom(s))),
  useDemo: (dir: string) => as<ProviderState>(Providers.UseDemo(dir)),
  disconnect: () => as<ProviderState>(Providers.Disconnect()),
  setLimits: (l: Limits) => as<ProviderState>(Providers.SetLimits(desktop.Limits.createFrom(l))),
  sendTest: (to: string) => Providers.SendTest(to),

  loadData: (path: string) => as<TableData>(Campaign.LoadData(path)),
  validate: (r: CampaignRequest) => as<ValidationResult>(Campaign.Validate(desktop.CampaignRequest.createFrom(r))),
  snapshot: () => as<ValidationResult>(Campaign.Snapshot()),
  previewMessage: (r: CampaignRequest, index: number) =>
    as<MessagePreview>(Campaign.PreviewMessage(desktop.CampaignRequest.createFrom(r), index)),
  startBatch: () => Campaign.StartBatch(),
  retryFailed: () => Campaign.RetryFailed(),
  cancelBatch: () => Campaign.CancelBatch(),
  exportReport: () => Campaign.ExportReport(),

  previewRename: (r: RenameRequest) => as<RenamePlan>(Rename.Preview(desktop.RenameRequest.createFrom(r))),
  applyRename: (r: RenameRequest) => as<RenamePlan>(Rename.Apply(desktop.RenameRequest.createFrom(r))),
  undoRename: () => as<RenamePlan>(Rename.Undo()),
};

export const events = {
  onItem: (fn: (e: ItemEvent) => void) => EventsOn("batch:item", fn),
  onLog: (fn: (l: LogLine) => void) => EventsOn("batch:log", fn),
  onDone: (fn: (d: DoneEvent) => void) => EventsOn("batch:done", fn),
  onFatal: (fn: (msg: string) => void) => EventsOn("batch:fatal", fn),
};

// Los errores de Go llegan como string, no como Error.
export function errorText(err: unknown): string {
  if (typeof err === "string") return err;
  if (err instanceof Error) return err.message;
  return String(err);
}
