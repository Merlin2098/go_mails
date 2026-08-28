# BoletaMailer — Spec de UI (MVP visual, primer corte)

Estado: aprobado para implementación.

Reemplaza, para esta iteración, el layout de 3 tabs de `docs/boleta_mailer_mvp_context.md` §18 por
**2 tabs**. El tab "Historial" queda fuera de este corte — se cubre parcialmente con el reporte
Excel post-envío (ver §3). Auditoría de lotes pasados como pantalla navegable es trabajo futuro,
no de este spec.

---

## 1. Layout general

```text
+--------------------------------------------------------------+
| BoletaMailer                                                 |
+------------------+---------------------------------------------+
| [Renombrar]       |                                           |
| [Enviar]           |            <contenido del tab activo>     |
+------------------+---------------------------------------------+
```

- Navegación por tabs, sin router — es una app de 2 pantallas, un router sería
  complejidad no justificada (agents.md §8).
- Estado de cada tab vive en React (`useState`/`useReducer`); el backend Go no conoce
  qué tab está activo — eso es presentación pura (agents.md §21).

---

## 2. Tab 1 — Renombrar

### Componentes

| Componente | Tipo | Comportamiento |
|---|---|---|
| Selector de carpeta | Botón "Seleccionar carpeta" | Abre diálogo nativo de Windows vía Wails runtime (`OpenDirectoryDialog`). Muestra la ruta elegida junto al botón. |
| Vista previa | Tabla (Original → Nuevo) | Se puebla tras seleccionar carpeta. Backend calcula el renombrado propuesto; React solo lo lista. |
| Botón Renombrar | Botón primario | Deshabilitado hasta que haya una carpeta seleccionada y preview generado. Ejecuta el renombrado real tras confirmación. |
| Barra de progreso | Progress bar | Visible solo durante la ejecución del renombrado. Con 50-500 archivos el renombrado es rápido pero no instantáneo — igual se emite progreso incremental para consistencia con el tab Enviar. |
| Resultado | Texto/lista de errores | Tras terminar: cuántos renombrados OK, cuántos con error (archivo bloqueado, nombre duplicado, etc.). |

### Flujo

```text
Seleccionar carpeta
       |
       v
Backend lista archivos + genera preview de renombrado
       |
       v
Usuario revisa preview (Original | Nuevo)
       |
       v
Click "Renombrar"
       |
       v
Progreso en tiempo real
       |
       v
Resultado (OK / errores)
```

### Contrato Go ↔ React (bindings)

```go
// app/application — firmas de casos de uso que este tab consume
func (a *App) SelectFolder() (string, error)
func (a *App) PreviewRename(folderPath string) ([]RenamePreviewItem, error)
func (a *App) ConfirmRename(items []RenamePreviewItem) error
```

```go
type RenamePreviewItem struct {
    OriginalName string
    NewName      string
    FullPath     string
}
```

Progreso del renombrado se emite vía Wails events (`runtime.EventsEmit`), no polling —
mismo mecanismo que el tab Enviar (ver §4), para no duplicar dos formas de progreso.

---

## 3. Tab 2 — Enviar

### Componentes

| Componente | Tipo | Comportamiento |
|---|---|---|
| Selector de proveedor | Radio group | Elige entre Outlook y Gmail como transporte de envío (docs/specs/POC_Outlook_Hotmail_Desktop.md, docs/specs/POC_Gmail_Desktop.md). Determina qué sección de conexión se muestra debajo. |
| Botón "Probar conexión con Outlook" | Botón secundario | Visible solo con proveedor Outlook. Ejecuta el diagnóstico de Outlook Classic (context.md §6/§20) bajo demanda, no automático al arranque. Muestra OK/error y, si OK, el correo configurado en el perfil de Outlook Desktop detectado. Al tener éxito, dispara automáticamente la carga del selector de cuenta remitente (ver fila siguiente) — es la única acción de este tab con conexión real a backend en esta iteración (ver Nota de implementación más abajo). |
| Selector de cuenta remitente (Outlook) | Radio group | Lista todas las cuentas configuradas en el perfil local de Outlook Classic (`Namespace.Accounts` vía COM, docs/specs/POC_Outlook_Hotmail_Desktop.md). El usuario debe elegir una explícitamente — la app nunca asume la cuenta por defecto de Outlook. Solo visible cuando la prueba de conexión tuvo éxito. |
| Formulario de cuenta Gmail | Campos correo + contraseña de aplicación | Visible solo con proveedor Gmail. `TestGmailConnection` valida las credenciales por SMTP sin enviar nada; al tener éxito, "Conectar" llama `SelectGmailAccount` y la sección pasa a mostrar estado "Conectado" + botón "Desconectar" (docs/specs/POC_Gmail_Desktop.md). |
| Botón "Enviar prueba" (Gmail) | Botón secundario | Visible tras conectar la cuenta Gmail. Envía un único correo de prueba (`SendTestGmail`) a la dirección indicada, independiente del lote de boletas. |
| Selector de carpeta de boletas | Botón "Seleccionar carpeta" | Diálogo nativo de carpeta. Muestra ruta elegida. Es la carpeta que el motor de resolución usa junto al Excel para asociar `DNI → archivo` (docs/specs/docs_enrutamiento.md §5). |
| Selector de Excel | Botón "Seleccionar Excel" | Diálogo nativo, filtra `.xlsx`. Muestra ruta elegida. |
| Resumen de validación | Texto/tabla | Tras seleccionar Excel y carpeta: total de registros, válidos, inválidos, con detalle de errores (context.md §11), incluyendo documentos no encontrados en la carpeta seleccionada. |
| Botón Enviar | Botón primario | Deshabilitado si: no hay Excel cargado, no hay carpeta seleccionada, hay 0 registros válidos, o el proveedor seleccionado no tiene una cuenta lista (Outlook validado + cuenta elegida, o Gmail conectado). |
| Barra de progreso | Progress bar | Enviados/total, contador de éxitos y fallos, nombre del registro actual — igual que context.md §14. |
| Botón "Reintentar fallidos" | Botón secundario | Aparece solo cuando el lote terminó con ≥1 fallo. Reenvía únicamente los ítems en estado `FAILED` (context.md §15). |
| Botón "Generar reporte Excel" | Botón secundario | Aparece cuando el lote terminó (sent o failed, no en progreso). Exporta el resultado del envío a un `.xlsx` vía diálogo "Guardar como". |
| Botón Cancelar | Botón secundario/destructivo | Visible solo durante el envío. Cancelación cooperativa (context.md §22). |

### Flujo

```text
Probar conexión Outlook ---> OK (+ correo detectado) / Error
       |
       v
Seleccionar cuenta remitente (lista de cuentas de Outlook Classic)
       |
       v
Seleccionar carpeta de boletas
       |
       v
Seleccionar Excel
       |
       v
Backend valida el lote completo (sin efectos secundarios) —
resuelve DNI -> archivo dentro de la carpeta seleccionada
       |
       v
Resumen: N válidos / M inválidos
       |
       v
Click "Enviar"  (bloqueado si Outlook no fue validado, falta cuenta/carpeta/Excel, o 0 válidos)
       |
       v
Progreso en tiempo real (enviados, fallidos, actual)
       |
       +--> Cancelar (cooperativo)
       |
       v
Fin del lote
       |
       +--> "Reintentar fallidos" (si failed > 0)
       |
       v
"Generar reporte Excel"
```

### Contrato Go ↔ React (bindings)

```go
func (a *App) TestOutlookConnection() (OutlookStatus, error)
func (a *App) ListOutlookAccounts() ([]OutlookAccount, error)
func (a *App) SelectOutlookAccount(smtpAddress string)
func (a *App) TestGmailConnection(email string, appPassword string) (GmailStatus, error)
func (a *App) SelectGmailAccount(email string, appPassword string)
func (a *App) DisconnectGmailAccount()
func (a *App) SendTestGmail(to string) error
func (a *App) SelectDocumentsFolder() (string, error)
func (a *App) SelectExcel() (string, error)
func (a *App) ValidateBatch(excelPath string, folderPath string) (BatchValidationSummary, error)
func (a *App) StartBatch() error          // async, progreso vía eventos
func (a *App) CancelBatch() error
func (a *App) RetryFailed() error         // async, progreso vía eventos
func (a *App) ExportReport(savePath string) error
```

```go
type OutlookStatus struct {
    Available bool
    Email     string // dirección detectada en el perfil de Outlook Desktop; vacío si Available=false
    Detail    string
}

type OutlookAccount struct {
    DisplayName string
    SmtpAddress string
}

type GmailStatus struct {
    Available bool
    Email     string
    Detail    string
}

type BatchValidationSummary struct {
    Total       int
    Valid       int
    Invalid     int
    Errors      []ValidationError
}

type ValidationError struct {
    Row     int
    Field   string
    Message string
}
```

### Nota de implementación (actualizado 2026-08-26)

De los bindings de este tab, tienen **implementación real**:

- `TestOutlookConnection` — contra Outlook COM (`app/infrastructure/outlook`).
- `ListOutlookAccounts` — enumera `Namespace.Accounts` vía COM
  (`outlook.ListAccounts`), sin asumir ninguna cuenta por defecto
  (docs/specs/POC_Outlook_Hotmail_Desktop.md). `SelectOutlookAccount` guarda
  la elección en `App.selectedAccount`, solo en memoria para la sesión
  actual. `StartBatch`/`RetryFailed` devuelven `errNoAccountSelected` si se
  llaman antes de que el usuario elija.
- `TestGmailConnection`, `SelectGmailAccount`, `DisconnectGmailAccount`, `SendTestGmail` —
  autenticación por SMTP (`app/infrastructure/gmail`, `smtp.gmail.com:587` con STARTTLS y
  una "contraseña de aplicación" de Google en vez de OAuth — ver docs/specs/POC_Gmail_Desktop.md,
  revisión). `SelectGmailAccount` guarda las credenciales en memoria (`App.gmailEmail`/
  `gmailAppPassword`, nunca en disco ni en logs) y marca Gmail como proveedor activo.
  `DisconnectGmailAccount` limpia esas credenciales y, si Gmail era el proveedor activo,
  vuelve a exigir seleccionar uno antes del próximo envío.
- Selección de proveedor: `App.activeProvider` (Outlook/Gmail/ninguno) se fija al llamar
  `SelectOutlookAccount` o `SelectGmailAccount` — no hay un binding "elegir proveedor"
  aparte, el propio flujo de conexión de cada proveedor lo determina.
  `StartBatch`/`RetryFailed` devuelven `errNoAccountSelected` si `activeProvider` está
  vacío.
- `SelectDocumentsFolder`, `SelectExcel` — diálogos nativos de Wails (`runtime.OpenDirectoryDialog`/`OpenFileDialog`).
- `ValidateBatch` — lee el Excel real (`app/infrastructure/excel`, Excelize), resuelve
  documentos por DNI (`app/infrastructure/filesystem`) y aplica las reglas de
  validación (`app/application.ValidateBatch`). El `domain.Batch` resultante queda en
  memoria en `App.currentBatch`, listo para que `StartBatch` lo consuma.
- `StartBatch`, `CancelBatch` — `runBatchWorker` construye el `domain.MailSender` según
  `activeProvider`:
  - Outlook: `app/infrastructure/outlook.Adapter` (crea `MailItem`, selecciona la cuenta
    remitente vía `MailItem.SendUsingAccount`, resuelta contra `Namespace.Accounts` a
    partir del `SmtpAddress` elegido por el usuario, adjunta el documento resuelto,
    `Send()` inmediato). Corre con `runtime.LockOSThread()` — necesario porque COM es
    apartment-threaded: `Connect`/`Send`/`Close` deben ocurrir en el mismo hilo del
    sistema operativo, y sin el lock el scheduler de Go puede migrar la goroutine entre
    llamadas (causó `RPC_E_WRONG_THREAD` en la práctica antes del fix).
  - Gmail: `app/infrastructure/gmail.Adapter` (`net/smtp` sobre `smtp.gmail.com:587`,
    STARTTLS, autenticado con la contraseña de aplicación). No usa `LockOSThread` — es
    HTTP/SMTP puro sobre `net/smtp`, sin el apartment-threading que exige COM.
  Worker único secuencial (`app/application.RunBatch`, context.md §13) corriendo en su
  propia goroutine para cualquiera de los dos proveedores. Progreso emitido por el
  evento Wails `batch:progress`; un error fatal de conexión (p. ej. Outlook no
  disponible al iniciar el worker) se emite por `batch:fatal-error`.
  Cancelación cooperativa: el ítem en curso termina, el resto queda `CANCELLED`.
  Rutas de adjunto que excedan MAX_PATH (260 caracteres) se envuelven con el prefijo
  `\\?\`/`\\?\UNC\` (`app/infrastructure/winpath.ExtendedLengthPrefix`) antes de
  pasarlas a `Attachments.Add` — la única API tocada por este proyecto que no soporta
  rutas largas de forma transparente (a diferencia de `os.*`, soportado desde Go 1.19).
  El `.exe` también declara `longPathAware` en su manifest
  (`build/windows/wails.exe.manifest`) — necesario en el lado de la app, pero no
  suficiente por sí solo: la máquina destino también debe tener
  `LongPathsEnabled=1` en `HKLM\SYSTEM\CurrentControlSet\Control\FileSystem`, fuera
  del control de la aplicación.
- `RetryFailed` — reutiliza el mismo worker (`runBatchWorker`) tras
  `application.PrepareRetry`, que resetea a `READY` únicamente los ítems `FAILED` del
  `currentBatch` en memoria — nunca reenvía los ya `SENT` (context.md §15). El frontend
  no necesita conocer la lista de ítems fallidos: el conteo ya viaja en el último evento
  `batch:progress` (`failed`), suficiente para habilitar el botón y mostrar el número.
- `ExportReport` — abre el diálogo nativo "Guardar como" (`runtime.SaveFileDialog`) y
  escribe el `currentBatch` a `.xlsx` vía `app/infrastructure/excel.WriteReport`
  (columnas: Nombre, DNI, Correo, Archivo, Estado, Error). No depende de Outlook —
  puede llamarse tras `StartBatch` o `RetryFailed`, o incluso tras una cancelación.

Sigue servido por `frontend/src/services/mockApi.ts`:

- Tab Renombrar completo — sigue mockeado; su implementación real queda pendiente
  como módulo aparte (fuera de esta iteración).

El Excel de referencia usado para validar en desarrollo es `data/Plantilla_Envios.xlsx`
(hoja de destinatarios `DNI/NOMBRE/CORREO` + hoja de mensaje `Asunto/MENSAJE` con
placeholder `{NOMBRE}` — ver `app/domain/message_template.go`), con documentos de
prueba en `data/Boletas/`. **Cuidado:** `StartBatch` contra este Excel envía correos
reales a las direcciones de esas dos filas — no es un modo simulación (context.md §19,
aún no implementado).

### Progreso (evento Wails)

Payload igual al conceptual de context.md §14:

```json
{
  "batch_id": "20260826-103522",
  "total": 500,
  "processed": 310,
  "sent": 307,
  "failed": 3,
  "current": "Carlos Rodríguez"
}
```

React se suscribe con `EventsOn("batch:progress", ...)`. Un solo canal de eventos sirve
tanto para el envío inicial como para "Reintentar fallidos" — el payload ya distingue
por `total`/`processed` cuál subconjunto se está procesando.

### Reporte Excel

- Contiene, por fila: nombre, DNI, correo, archivo, estado final (`SENT`/`FAILED`),
  mensaje de error técnico si aplica.
- No incluye contenido de boletas ni datos salariales (context.md §23 — privacidad).
- Se genera a partir del estado en memoria del último lote — no requiere persistencia
  en disco previa, aunque el historial JSON (context.md §17) se sigue escribiendo en
  paralelo para auditoría de bajo nivel.

---

## 4. Fuera de alcance de este spec

- Tab Historial navegable (lotes anteriores, reintento desde un lote pasado que ya no
  está en memoria).
- Modo simulación (context.md §19) — no se pidió para este corte; se puede agregar como
  un checkbox en el tab Enviar sin romper este spec.
- Diagnóstico automático al arranque (context.md §20) — reemplazado por el botón manual
  "Probar conexión con Outlook".

Estas quedan explícitamente pendientes, no descartadas.
