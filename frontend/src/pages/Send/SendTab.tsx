import { useState } from "react";
import * as backendApi from "../../services/backendApi";
import type {
  BatchProgress,
  BatchValidationSummary,
  GmailStatus,
  OutlookAccount,
  OutlookStatus,
} from "../../types/batch";
import ProgressBar from "../../components/ProgressBar";

type SendPhase = "idle" | "sending" | "cancelled" | "finished" | "error";
type Provider = "outlook" | "gmail";

// isEmailLike is a lightweight client-side check, not a replacement for the
// backend's net/mail.ParseAddress validation (app.go) — it exists because
// type="email" inputs outside a <form> never trigger the browser's native
// constraint validation, so a value like "Hola" would otherwise reach the
// backend uncaught and surface as a raw SMTP rejection from Gmail (see
// logs/bug-gmail.log).
function isEmailLike(value: string): boolean {
  return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value.trim());
}

export default function SendTab() {
  const [provider, setProvider] = useState<Provider>("outlook");

  const [outlookStatus, setOutlookStatus] = useState<OutlookStatus | null>(null);
  const [testingOutlook, setTestingOutlook] = useState(false);

  const [accounts, setAccounts] = useState<OutlookAccount[] | null>(null);
  const [loadingAccounts, setLoadingAccounts] = useState(false);
  const [selectedAccount, setSelectedAccount] = useState("");

  const [gmailEmail, setGmailEmail] = useState("");
  const [gmailAppPassword, setGmailAppPassword] = useState("");
  const [gmailStatus, setGmailStatus] = useState<GmailStatus | null>(null);
  const [testingGmail, setTestingGmail] = useState(false);
  const [gmailConnected, setGmailConnected] = useState(false);
  const [gmailTestTo, setGmailTestTo] = useState("");
  const [sendingGmailTest, setSendingGmailTest] = useState(false);
  const [gmailTestResult, setGmailTestResult] = useState("");

  const [folderPath, setFolderPath] = useState("");
  const [excelPath, setExcelPath] = useState("");
  const [validation, setValidation] = useState<BatchValidationSummary | null>(null);
  const [validating, setValidating] = useState(false);

  const [phase, setPhase] = useState<SendPhase>("idle");
  const [progress, setProgress] = useState<BatchProgress | null>(null);
  const [fatalError, setFatalError] = useState("");
  const [exporting, setExporting] = useState(false);
  const [lastReportPath, setLastReportPath] = useState("");

  async function handleTestOutlook() {
    setTestingOutlook(true);
    const status = await backendApi.testOutlookConnection();
    setOutlookStatus(status);
    setTestingOutlook(false);
    if (status.available) {
      await loadAccounts();
    }
  }

  async function loadAccounts() {
    setLoadingAccounts(true);
    const list = await backendApi.listOutlookAccounts();
    setAccounts(list);
    setLoadingAccounts(false);
  }

  async function handleSelectAccount(smtpAddress: string) {
    setSelectedAccount(smtpAddress);
    await backendApi.selectOutlookAccount(smtpAddress);
  }

  async function handleOpenGmailAppPasswordsPage() {
    await backendApi.openGmailAppPasswordsPage();
  }

  async function handleTestGmail() {
    setTestingGmail(true);
    const status = await backendApi.testGmailConnection(gmailEmail, gmailAppPassword);
    setGmailStatus(status);
    setTestingGmail(false);
  }

  async function handleConnectGmail() {
    if (!gmailStatus?.available) return;
    await backendApi.selectGmailAccount(gmailEmail, gmailAppPassword);
    setGmailConnected(true);
  }

  async function handleDisconnectGmail() {
    await backendApi.disconnectGmailAccount();
    setGmailConnected(false);
    setGmailStatus(null);
    setGmailAppPassword("");
  }

  async function handleSendTestGmail() {
    setSendingGmailTest(true);
    setGmailTestResult("");
    try {
      await backendApi.sendTestGmail(gmailTestTo);
      setGmailTestResult("Correo de prueba enviado.");
    } catch (err) {
      setGmailTestResult(err instanceof Error ? err.message : String(err));
    }
    setSendingGmailTest(false);
  }

  async function handleSelectFolder() {
    const path = await backendApi.selectDocumentsFolder();
    if (!path) return; // usuario canceló el diálogo
    setFolderPath(path);
    setValidation(null);
    setPhase("idle");
    if (excelPath) {
      await runValidation(excelPath, path);
    }
  }

  async function handleSelectExcel() {
    const path = await backendApi.selectExcel();
    if (!path) return; // usuario canceló el diálogo
    setExcelPath(path);
    setValidation(null);
    setPhase("idle");
    if (folderPath) {
      await runValidation(path, folderPath);
    }
  }

  async function runValidation(excel: string, folder: string) {
    setValidating(true);
    const summary = await backendApi.validateBatch(excel, folder);
    setValidation(summary);
    setValidating(false);
  }

  async function runAndAwait(action: () => Promise<void>) {
    setPhase("sending");
    setFatalError("");
    setLastReportPath("");
    try {
      await action();
      setPhase("finished");
    } catch (err) {
      setFatalError(err instanceof Error ? err.message : String(err));
      setPhase("error");
    }
  }

  async function handleSend() {
    if (!validation) return;
    setProgress({ batchId: "", total: validation.valid, processed: 0, sent: 0, failed: 0, current: "" });
    await runAndAwait(() => backendApi.startBatch((p) => setProgress(p)));
  }

  async function handleRetryFailed() {
    if (!progress) return;
    setProgress({ ...progress, total: progress.failed, processed: 0, sent: 0, failed: 0, current: "" });
    await runAndAwait(() => backendApi.retryFailed((p) => setProgress(p)));
  }

  async function handleCancel() {
    await backendApi.cancelBatch();
    setPhase("cancelled");
  }

  async function handleExportReport() {
    setExporting(true);
    const path = await backendApi.exportReport();
    if (path) setLastReportPath(path);
    setExporting(false);
  }

  const providerReady =
    provider === "outlook"
      ? outlookStatus?.available === true && selectedAccount !== ""
      : gmailConnected;

  const canSend =
    providerReady &&
    folderPath !== "" &&
    validation !== null &&
    validation.valid > 0 &&
    phase !== "sending";

  const showRetry = phase === "finished" && (progress?.failed ?? 0) > 0;
  const showExport = (phase === "finished" || phase === "cancelled") && progress !== null;

  return (
    <div className="tab-panel">
      <section className="field-group">
        <label>Proveedor de correo</label>
        <div className="field-row" role="radiogroup" aria-label="Selecciona proveedor de correo">
          <label className="radio-option">
            <input
              type="radio"
              name="provider"
              checked={provider === "outlook"}
              disabled={phase === "sending"}
              onChange={() => setProvider("outlook")}
            />
            Outlook
          </label>
          <label className="radio-option">
            <input
              type="radio"
              name="provider"
              checked={provider === "gmail"}
              disabled={phase === "sending"}
              onChange={() => setProvider("gmail")}
            />
            Gmail
          </label>
        </div>
      </section>

      {provider === "outlook" && (
        <>
          <section className="field-group">
            <div className="field-row">
              <button onClick={handleTestOutlook} disabled={testingOutlook}>
                {testingOutlook ? "Probando..." : "Probar conexión con Outlook"}
              </button>
              {outlookStatus && (
                <span className={outlookStatus.available ? "status-ok" : "status-error"}>
                  {outlookStatus.available ? "✓" : "✗"} {outlookStatus.detail}
                  {outlookStatus.available && outlookStatus.email ? ` (${outlookStatus.email})` : ""}
                </span>
              )}
            </div>
          </section>

          {outlookStatus?.available && (
            <section className="field-group">
              <label>Cuenta remitente</label>
              {loadingAccounts && <p>Cargando cuentas de Outlook...</p>}
              {!loadingAccounts && accounts !== null && accounts.length === 0 && (
                <p className="status-error">No se encontraron cuentas configuradas en Outlook.</p>
              )}
              {!loadingAccounts && accounts !== null && accounts.length > 0 && (
                <div className="field-row" role="radiogroup" aria-label="Selecciona cuenta de Outlook">
                  {accounts.map((acc) => (
                    <label key={acc.smtpAddress} className="radio-option">
                      <input
                        type="radio"
                        name="outlook-account"
                        value={acc.smtpAddress}
                        checked={selectedAccount === acc.smtpAddress}
                        disabled={phase === "sending"}
                        onChange={() => handleSelectAccount(acc.smtpAddress)}
                      />
                      {acc.displayName} ({acc.smtpAddress})
                    </label>
                  ))}
                </div>
              )}
            </section>
          )}
        </>
      )}

      {provider === "gmail" && (
        <section className="field-group">
          <label>Cuenta de Gmail</label>
          {!gmailConnected ? (
            <>
              <div className="field-row">
                <input
                  type="email"
                  placeholder="correo@gmail.com"
                  value={gmailEmail}
                  onChange={(e) => setGmailEmail(e.target.value)}
                />
                <input
                  type="password"
                  placeholder="Contraseña de aplicación (16 caracteres)"
                  value={gmailAppPassword}
                  onChange={(e) => setGmailAppPassword(e.target.value)}
                />
              </div>
              <p className="field-hint">
                Necesitas una contraseña de aplicación (no tu contraseña normal de Gmail).
                Requiere verificación en dos pasos activada en la cuenta.{" "}
                <button className="link-button" onClick={handleOpenGmailAppPasswordsPage} type="button">
                  Generar contraseña de aplicación
                </button>
              </p>
              {gmailEmail !== "" && !isEmailLike(gmailEmail) && (
                <p className="status-error">El correo no tiene un formato válido.</p>
              )}
              <div className="field-row">
                <button
                  onClick={handleTestGmail}
                  disabled={testingGmail || !isEmailLike(gmailEmail) || !gmailAppPassword}
                >
                  {testingGmail ? "Probando..." : "Probar conexión"}
                </button>
                {gmailStatus && (
                  <span className={gmailStatus.available ? "status-ok" : "status-error"}>
                    {gmailStatus.available ? "✓" : "✗"} {gmailStatus.detail}
                  </span>
                )}
                {gmailStatus?.available && (
                  <button className="btn-primary" onClick={handleConnectGmail}>
                    Conectar
                  </button>
                )}
              </div>
            </>
          ) : (
            <div className="field-row">
              <span className="status-ok">● Conectado ({gmailEmail})</span>
              <button onClick={handleDisconnectGmail} disabled={phase === "sending"}>
                Desconectar
              </button>
            </div>
          )}

          {gmailConnected && (
            <>
              <div className="field-row">
                <input
                  type="email"
                  placeholder="Enviar prueba a..."
                  value={gmailTestTo}
                  onChange={(e) => setGmailTestTo(e.target.value)}
                />
                <button
                  onClick={handleSendTestGmail}
                  disabled={sendingGmailTest || !isEmailLike(gmailTestTo)}
                >
                  {sendingGmailTest ? "Enviando..." : "Enviar prueba"}
                </button>
                {gmailTestResult && <span>{gmailTestResult}</span>}
              </div>
              {gmailTestTo !== "" && !isEmailLike(gmailTestTo) && (
                <p className="status-error">El correo del destinatario no tiene un formato válido.</p>
              )}
            </>
          )}
        </section>
      )}

      <section className="field-group">
        <label>Carpeta de boletas</label>
        <div className="field-row">
          <input type="text" readOnly value={folderPath} placeholder="Ninguna carpeta seleccionada" />
          <button onClick={handleSelectFolder} disabled={phase === "sending"}>
            Seleccionar carpeta
          </button>
        </div>
      </section>

      <section className="field-group">
        <label>Excel</label>
        <div className="field-row">
          <input type="text" readOnly value={excelPath} placeholder="Ningún Excel seleccionado" />
          <button onClick={handleSelectExcel} disabled={validating || phase === "sending"}>
            Seleccionar Excel
          </button>
        </div>
      </section>

      {validating && <p>Validando lote...</p>}

      {validation && (
        <section className="field-group validation-summary">
          <p>
            Registros: <strong>{validation.total}</strong> · Válidos:{" "}
            <strong>{validation.valid}</strong> · Inválidos: <strong>{validation.invalid}</strong>
          </p>
          {validation.errors.length > 0 && (
            <ul className="error-list">
              {validation.errors.map((err, i) => (
                <li key={i}>
                  Fila {err.row} ({err.field}): {err.message}
                </li>
              ))}
            </ul>
          )}
        </section>
      )}

      {progress && (phase === "sending" || phase === "finished" || phase === "cancelled") && (
        <section className="field-group">
          <ProgressBar
            processed={progress.processed}
            total={progress.total}
            running={phase === "sending"}
          />
          <p className="progress-detail">
            Enviados: {progress.sent} · Errores: {progress.failed}
            {progress.current && phase === "sending" ? ` · Actual: ${progress.current}` : ""}
          </p>
        </section>
      )}

      {phase === "cancelled" && <p className="status-error">Lote cancelado por el usuario.</p>}
      {phase === "finished" && <p className="status-ok">Lote finalizado.</p>}
      {phase === "error" && (
        <p className="status-error">No se pudo iniciar el envío: {fatalError}</p>
      )}

      <div className="actions">
        <button className="btn-primary" onClick={handleSend} disabled={!canSend}>
          Enviar
        </button>
        {phase === "sending" && (
          <button className="btn-danger" onClick={handleCancel}>
            Cancelar
          </button>
        )}
        {showRetry && (
          <button onClick={handleRetryFailed}>Reintentar fallidos ({progress?.failed})</button>
        )}
        {showExport && (
          <button onClick={handleExportReport} disabled={exporting}>
            {exporting ? "Generando..." : "Generar reporte Excel"}
          </button>
        )}
      </div>

      {lastReportPath && <p className="status-ok">Reporte guardado en: {lastReportPath}</p>}
    </div>
  );
}
