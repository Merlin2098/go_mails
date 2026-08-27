# BoletaMailer — Contexto del Proyecto MVP

## 1. Propósito

BoletaMailer es una aplicación desktop para Windows orientada a una MYPE que necesita automatizar el envío masivo de boletas de pago por correo electrónico utilizando Outlook Desktop.

El objetivo del MVP es reemplazar un proceso manual basado en:

1. Preparar/renombrar archivos de boletas.
2. Preparar un Excel con los destinatarios.
3. Abrir Outlook.
4. Crear y enviar manualmente cientos de correos con sus respectivos archivos adjuntos.
5. Hacer seguimiento manual de qué documentos fueron enviados y cuáles fallaron.

La aplicación debe permitir procesar lotes aproximados de **50 a 500 documentos**.

> El MVP no pretende convertirse en Adobe Sign ni en una plataforma SaaS. Es una herramienta desktop local, simple, portable y enfocada en automatizar un proceso operativo concreto.

---

# 2. Objetivos del MVP

## Objetivo principal

Permitir que un usuario pueda seleccionar un Excel y un conjunto de archivos, validar el lote y enviar las boletas automáticamente mediante Outlook Desktop mientras la aplicación continúa respondiendo.

## Objetivos funcionales

El MVP debe permitir:

- Seleccionar una carpeta de documentos.
- Renombrar documentos mediante una regla definida por los datos del Excel.
- Visualizar una vista previa antes de renombrar.
- Seleccionar un Excel plantilla.
- Leer registros de empleados.
- Validar los datos antes del envío.
- Validar que los archivos existan.
- Mostrar una vista previa del lote.
- Iniciar el envío.
- Mostrar progreso en tiempo real.
- Procesar los envíos en segundo plano.
- Mantener la aplicación usable durante el procesamiento.
- Evitar que Windows entre en suspensión mientras el lote está procesándose.
- Registrar éxitos y errores.
- Permitir identificar los documentos fallidos.
- Permitir reintentar únicamente los fallidos.
- Mantener historial local de los lotes procesados.

---

# 3. Alcance del MVP

## Incluido

### Gestión de documentos

- Selección de carpeta.
- Detección de archivos.
- Renombrado basado en datos del Excel.
- Vista previa de cambios.
- Confirmación antes de ejecutar el renombrado.

### Preparación del envío

- Carga de Excel `.xlsx`.
- Validación de columnas requeridas.
- Validación de email.
- Validación de DNI.
- Validación de existencia del archivo.
- Vista previa de registros.

### Envío

- Integración con Outlook Classic para Windows.
- Creación de mensajes.
- Asunto configurable o definido por la aplicación.
- Adjuntar la boleta correspondiente.
- Envío secuencial.
- Procesamiento en background.
- Barra de progreso.
- Estado individual de cada registro.
- Manejo de errores.
- Reintento de fallidos.

### Operación

- Prevención temporal de suspensión del sistema durante el procesamiento.
- Logs.
- Historial local de lotes.
- Diagnóstico inicial del entorno.

## Fuera del alcance del MVP

No se implementará inicialmente:

- Firma digital.
- Adobe Sign / DocuSign.
- Portal web.
- Backend cloud.
- Base de datos remota.
- Multiusuario.
- Gestión de usuarios.
- Roles y permisos.
- Integración con ERP.
- Integración con SUNAT.
- Microsoft Graph como transporte principal.
- Servicio Windows permanente.
- Dashboard web.
- Distribución SaaS.

---

# 4. Restricción principal de distribución

La aplicación debe ser distribuible como:

> **Un único `.exe` autocontenido**, o como máximo un bundle autocontenido equivalente a un PyInstaller OneDir.

El usuario final no debería necesitar instalar:

- Python.
- Node.js.
- Go.
- Rust.
- npm.
- Dependencias adicionales de runtime.

El frontend utilizado durante desarrollo debe quedar embebido dentro del ejecutable final.

## Artefacto esperado

```text
BoletaMailer.exe
```

El estado persistente del usuario no debe escribirse dentro del directorio del ejecutable.

---

# 5. Stack tecnológico propuesto

## Backend / Core

**Go**

Motivos:

- Compilación a binario nativo.
- Excelente soporte de concurrencia.
- Buen desempeño para procesamiento de lotes.
- Fácil distribución en Windows.
- Sin necesidad de runtime externo.
- Adecuado para interacción con APIs nativas de Windows.
- Buen encaje para procesos background.

## UI

**Wails + React**

La UI será una aplicación React embebida dentro del ejecutable mediante Wails.

No se utilizará una arquitectura HTTP local entre frontend y backend.

La comunicación será mediante bindings de Wails:

```text
React
  |
  | Wails bindings
  v
Go
```

## Excel

Utilizar una librería Go para lectura/escritura de `.xlsx`.

Candidato inicial:

- Excelize

No utilizar Python, pandas ni openpyxl.

## Outlook

MVP:

**Outlook Classic para Windows mediante COM/OOM.**

Esto es una dependencia externa del entorno y no puede empaquetarse dentro del ejecutable.

## Windows

Utilizar APIs nativas de Windows cuando sea necesario, particularmente para:

- Prevención temporal de suspensión.
- Detección/diagnóstico del entorno.
- Integración con funcionalidades del sistema.

---

# 6. Dependencias externas

Aunque la aplicación sea self-contained, existen dependencias que necesariamente pertenecen al entorno:

```text
BoletaMailer.exe
      |
      +-- Windows
      |
      +-- Outlook Classic
      |
      +-- Microsoft 365 / Exchange
      |
      +-- Archivos del usuario
      |
      +-- Excel del usuario
```

La aplicación debe realizar un diagnóstico antes de iniciar el proceso.

## Diagnóstico mínimo

Verificar:

- Sistema operativo Windows.
- Disponibilidad de Outlook Classic.
- Outlook correctamente configurado.
- Posibilidad de automatizar Outlook.
- Acceso a las carpetas seleccionadas.
- Permisos de lectura.
- Permisos de escritura cuando corresponda.

Si Outlook no está disponible, el usuario debe recibir un error claro antes de comenzar el envío.

---

# 7. Arquitectura lógica

```text
                         React UI
                            |
                       Wails bindings
                            |
                            v
                    +---------------+
                    |    Go Core    |
                    +---------------+
                            |
          +-----------------+------------------+
          |                 |                  |
          v                 v                  v
    Excel Service      File Service      Batch Service
                                             |
                                             v
                                           Queue
                                             |
                                             v
                                      Outlook Worker
                                             |
                                             v
                                      Outlook Adapter
                                             |
                                             v
                                      Outlook Classic
```

## Principio importante

La lógica de negocio no debe depender directamente de Outlook.

El dominio debe trabajar contra una abstracción:

```go
type MailSender interface {
    Send(message Message) error
}
```

El MVP implementará:

```text
MailSender
    |
    v
OutlookAdapter
    |
    v
Outlook COM
```

Esto permitirá implementar posteriormente:

```text
MailSender
    |
    +-- OutlookAdapter
    |
    +-- GraphAdapter
```

sin modificar el procesamiento principal.

---

# 8. Arquitectura de capas

## Domain

Contiene las entidades y reglas del negocio.

Ejemplos:

```text
Employee
Document
EmailMessage
Batch
BatchItem
SendResult
```

No debe conocer:

- React.
- Wails.
- Outlook.
- COM.
- Windows API.
- Excelize.

## Application

Contiene los casos de uso:

```text
RenameDocuments
ValidateBatch
LoadExcel
PreviewBatch
StartBatch
CancelBatch
RetryFailed
GetBatchStatus
GetHistory
```

## Infrastructure

Implementa detalles externos:

```text
Excel
Filesystem
Outlook
Windows
Logging
Persistence
```

## UI

React únicamente presenta información y dispara comandos.

No debe contener reglas críticas de negocio.

---

# 9. Estructura inicial del repositorio

Propuesta:

```text
boleta-mailer/
|
├── app/
│   ├── domain/
│   │   ├── employee.go
│   │   ├── document.go
│   │   ├── batch.go
│   │   ├── message.go
│   │   └── result.go
│   │
│   ├── application/
│   │   ├── batch_service.go
│   │   ├── validation_service.go
│   │   ├── rename_service.go
│   │   └── history_service.go
│   │
│   └── infrastructure/
│       ├── excel/
│       ├── filesystem/
│       ├── outlook/
│       ├── windows/
│       ├── logging/
│       └── persistence/
│
├── frontend/
│   ├── src/
│   │   ├── components/
│   │   ├── pages/
│   │   │   ├── Rename/
│   │   │   ├── Send/
│   │   │   └── History/
│   │   ├── services/
│   │   └── types/
│   │
│   └── package.json
│
├── main.go
├── go.mod
├── wails.json
└── README.md
```

---

# 10. Modelo de datos del Excel

La plantilla inicial tendrá:

| Columna | Obligatoria | Descripción |
|---|---:|---|
| nombre | Sí | Nombre completo del empleado |
| dni | Sí | DNI del empleado |
| correo | Sí | Correo electrónico de destino |
| ruta_archivo | Sí | Ruta de la boleta |

Ejemplo:

```text
nombre              dni       correo                  ruta_archivo
Juan Pérez          12345678  juan@empresa.com        C:\Boletas\12345678.pdf
Ana López           87654321  ana@empresa.com         C:\Boletas\87654321.pdf
```

El nombre real de las columnas deberá definirse como parte del contrato del MVP y mantenerse estable.

---

# 11. Validación

El sistema debe validar el lote completo antes de iniciar cualquier envío.

## Validaciones mínimas

### Excel

- Archivo `.xlsx` válido.
- Hoja esperada disponible.
- Columnas obligatorias presentes.
- Filas no vacías.

### Datos

- Nombre no vacío.
- DNI válido según las reglas definidas.
- Correo con formato válido.
- Ruta de archivo no vacía.

### Filesystem

- Archivo existe.
- Archivo es accesible.
- Archivo tiene extensión permitida.
- Archivo puede abrirse/adjuntarse.

## Resultado esperado

Ejemplo:

```text
500 registros

493 válidos
4 correos inválidos
2 archivos inexistentes
1 DNI inválido

Estado: BLOQUEADO
```

No se debe comenzar el envío si existen errores críticos de validación.

---

# 12. Modelo de estados

Cada elemento del lote debe tener un estado.

```text
PENDING
   |
   v
VALIDATING
   |
   +----> INVALID
   |
   v
READY
   |
   v
SENDING
   |
   +----> FAILED
   |
   v
SENT
```

Estados adicionales:

```text
CANCELLED
```

El estado del lote puede derivarse de los estados individuales.

---

# 13. Procesamiento en background

El usuario no debe quedar bloqueado mientras se procesan los correos.

La UI inicia:

```text
StartBatch()
```

El Go backend inicia el procesamiento en segundo plano.

Arquitectura:

```text
UI
 |
 | StartBatch()
 v
Go Application Service
 |
 v
Batch Processor
 |
 v
Queue
 |
 v
Single Outlook Worker
 |
 v
Outlook
```

## Decisión importante

No ejecutar 500 envíos concurrentes.

Se utilizará inicialmente:

> **Un único worker dedicado a Outlook.**

Motivos:

- Outlook COM puede presentar problemas con concurrencia.
- Se reduce el riesgo de race conditions.
- Se simplifica el manejo de errores.
- El cuello de botella está en Outlook/Microsoft 365, no en Go.
- 50–500 documentos no requieren paralelismo agresivo.

---

# 14. Progreso

El backend debe emitir eventos de progreso hacia React.

Ejemplo conceptual:

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

La UI mostrará:

```text
Procesando...

████████████░░░░░░░░ 62%

Procesados: 310 / 500
Enviados:   307
Errores:      3

Actual:
Carlos Rodríguez
```

---

# 15. Reintentos

El usuario debe poder reintentar únicamente los elementos que fallaron.

Ejemplo:

```text
500 documentos

493 enviados
7 fallidos
```

La acción:

```text
Reintentar errores
```

debe procesar solamente los 7 elementos fallidos.

No debe volver a enviar automáticamente los 493 documentos exitosos.

---

# 16. Prevención de suspensión

Durante un lote activo, Windows no debe entrar en suspensión por inactividad.

Se utilizará la API:

```text
SetThreadExecutionState()
```

El comportamiento esperado es:

```text
Inicio del lote
      |
      v
ES_SYSTEM_REQUIRED
      |
      v
Procesamiento
      |
      v
Fin del lote
      |
      v
Restaurar estado normal
```

Inicialmente se recomienda evitar forzar la pantalla permanentemente encendida.

Objetivo:

> Mantener el sistema despierto, pero permitir que la pantalla se apague normalmente cuando sea posible.

El estado debe restaurarse siempre al finalizar, cancelar o fallar el lote.

---

# 17. Historial

El MVP no necesita una base de datos.

Se utilizará persistencia local.

Ubicación recomendada:

```text
%LOCALAPPDATA%\BoletaMailer\
```

Estructura:

```text
BoletaMailer/
├── config.json
├── logs/
│   └── YYYY-MM-DD.log
└── history/
    └── YYYYMMDD-HHMMSS.json
```

Ejemplo conceptual:

```json
{
  "batch_id": "20260826-103522",
  "started_at": "2026-08-26T10:35:22",
  "finished_at": "2026-08-26T11:14:07",
  "total": 500,
  "sent": 493,
  "failed": 7
}
```

SQLite queda fuera del MVP salvo que aparezca una necesidad real.

---

# 18. UI del MVP

## Tab 1 — Renombrar

Funciones:

1. Seleccionar carpeta.
2. Leer archivos.
3. Asociar archivos con datos.
4. Generar preview.
5. Mostrar nombre actual/nuevo.
6. Confirmar.
7. Renombrar.
8. Mostrar resultado.

Conceptualmente:

```text
+------------------------------------------------------+
| BoletaMailer                                          |
+------------------+-----------------------------------+
| Renombrar        |                                   |
| Enviar           | Carpeta                           |
| Historial        | [ C:\Boletas ] [Seleccionar]      |
|                  |                                   |
|                  | Vista previa                     |
|                  |                                   |
|                  | Original       Nuevo              |
|                  | BOLETA123.pdf  12345678-Juan.pdf  |
|                  | BOLETA456.pdf  87654321-Ana.pdf   |
|                  |                                   |
|                  | [Renombrar]                       |
+------------------+-----------------------------------+
```

## Tab 2 — Enviar

Flujo:

```text
Seleccionar Excel
        |
        v
Validar
        |
        v
Preview
        |
        v
Confirmar
        |
        v
Enviar
        |
        v
Progress
```

Debe mostrar:

- Archivo Excel.
- Cantidad de registros.
- Cantidad válida.
- Cantidad inválida.
- Botón iniciar.
- Botón cancelar.
- Barra de progreso.
- Contadores.
- Registro del elemento actual.

## Tab 3 — Historial

Debe permitir:

- Ver lotes anteriores.
- Ver resumen.
- Ver errores.
- Identificar documentos enviados.
- Reintentar fallidos.

---

# 19. Modo simulación

El MVP debe considerar un modo:

```text
[ ] Simulación
```

En modo simulación:

- Se valida el Excel.
- Se validan los archivos.
- Se genera el lote.
- Se muestra qué correos serían enviados.
- No se ejecuta el envío real mediante Outlook.

Esto permitirá validar lotes sin riesgo.

---

# 20. Diagnóstico inicial

Al iniciar la aplicación:

```text
BoletaMailer
      |
      v
System Check
      |
      +-- Windows ............. OK
      +-- Outlook Classic ..... OK
      +-- Outlook Profile ..... OK
      +-- File Access ......... OK
      +-- Application State ... OK
      |
      v
Ready
```

Si falla una dependencia crítica:

```text
No se puede iniciar el envío.

Outlook Classic no está disponible.

La aplicación requiere Outlook Classic
configurado en este equipo.
```

---

# 21. Manejo de errores

Los errores deben ser tratados individualmente.

Ejemplo:

```text
Juan Pérez
    -> SENT

Ana López
    -> SENT

Pedro Díaz
    -> FAILED
       Outlook error

Carlos Ruiz
    -> SENT
```

Un error individual no debe detener automáticamente todo el lote, salvo que se trate de una condición global:

- Outlook se cerró.
- Outlook dejó de responder.
- Se perdió la sesión.
- Error fatal del sistema.
- Error de autenticación.
- Aplicación cancelada.

Debe diferenciarse:

```text
Item Error
```

de:

```text
Batch Fatal Error
```

---

# 22. Cancelación

El usuario debe poder cancelar un lote.

La cancelación debe ser cooperativa:

```text
Usuario
   |
   v
Cancelar
   |
   v
Cancellation Token
   |
   v
Worker termina operación actual
   |
   v
No toma nuevos elementos
   |
   v
Batch = CANCELLED
```

No se debe matar abruptamente el proceso de Outlook ni el proceso de la aplicación para cancelar.

---

# 23. Seguridad y privacidad

Las boletas contienen información sensible.

Principios:

- No enviar documentos a servicios externos en el MVP.
- No subir archivos a la nube.
- No almacenar contenido de boletas en logs.
- No registrar información innecesaria.
- Los logs deben almacenar únicamente metadata operacional.
- El historial debe almacenarse localmente.
- No guardar contraseñas.
- No guardar tokens de autenticación.
- Los archivos originales no deben copiarse innecesariamente.

Los logs deben evitar datos como:

```text
❌ contenido de la boleta
❌ información salarial
❌ DNI completo cuando no sea necesario
❌ cuerpo completo del correo
```

Preferir:

```text
✓ batch_id
✓ timestamp
✓ estado
✓ nombre de archivo
✓ resultado
✓ código/mensaje de error técnico
```

---

# 24. Outlook Adapter

El dominio no debe conocer COM.

La infraestructura debe encapsularlo:

```text
MailSender
    ^
    |
OutlookAdapter
    |
    +-- Connect()
    +-- Validate()
    +-- Send()
    +-- Close()
```

El código relacionado con:

- COM.
- Outlook.Application.
- MailItem.
- Attachments.
- Send.

debe permanecer dentro de:

```text
app/infrastructure/outlook/
```

Esto permitirá sustituirlo posteriormente.

---

# 25. Evolución futura

La arquitectura debe permitir evolucionar:

```text
                  MailSender
                     |
          +----------+----------+
          |                     |
          v                     v
   Outlook Classic        Microsoft Graph
       Adapter                Adapter
```

El MVP utilizará:

```text
Outlook Classic
```

pero Microsoft Graph puede convertirse posteriormente en el mecanismo de envío.

No se debe implementar Graph en el MVP salvo que Outlook Classic no sea viable en el entorno objetivo.

---

# 26. Distribución

## MVP

Compilación:

```text
wails build
```

Resultado esperado:

```text
build/bin/BoletaMailer.exe
```

La aplicación debe ser ejecutable en una máquina Windows objetivo sin instalar el entorno de desarrollo.

## Verificación

El artefacto final debe probarse en un equipo limpio.

Checklist conceptual:

```text
- [ ] Windows compatible
- [ ] No Go instalado
- [ ] No Node instalado
- [ ] No Python instalado
- [ ] Ejecutar BoletaMailer.exe
- [ ] Detectar Outlook
- [ ] Seleccionar Excel
- [ ] Validar lote
- [ ] Procesar lote pequeño
- [ ] Procesar lote real
- [ ] Confirmar logs
- [ ] Confirmar historial
- [ ] Confirmar recuperación ante errores
```

---

# 27. Criterios de aceptación del MVP

## Renombrado

- El usuario puede seleccionar una carpeta.
- El sistema detecta los archivos.
- Se genera preview.
- El usuario confirma antes de modificar.
- Los archivos se renombran correctamente.
- Los errores no producen pérdida de archivos.

## Excel

- Se puede seleccionar un `.xlsx`.
- Se validan columnas.
- Se validan registros.
- Se identifican errores antes de enviar.

## Envío

- Se puede iniciar un lote.
- Outlook Classic es utilizado para enviar.
- La UI permanece responsive.
- Los documentos se adjuntan correctamente.
- Cada registro obtiene un resultado.
- Los errores individuales no detienen automáticamente el lote.
- Se puede cancelar.
- Se puede reintentar fallidos.

## Sistema

- Windows no entra en suspensión durante el procesamiento.
- El estado energético se restaura al finalizar.
- La aplicación no requiere runtime externo.
- La aplicación funciona como ejecutable autocontenido.

## Auditoría

- Cada lote genera historial.
- Se registra fecha/hora.
- Se registra cantidad total.
- Se registra cantidad enviada.
- Se registra cantidad fallida.
- Los errores pueden identificarse.

---

# 28. Orden recomendado de desarrollo

## Fase 0 — Spike técnico

Objetivo: validar las dos partes más riesgosas.

1. Crear aplicación Wails mínima.
2. Compilar a `.exe`.
3. Probar lectura de `.xlsx`.
4. Probar conexión con Outlook Classic.
5. Crear un correo de prueba.
6. Adjuntar un PDF.
7. Enviar un único correo.
8. Probar `SetThreadExecutionState`.

No construir todavía la UI completa.

## Fase 1 — Core

Implementar:

- Domain models.
- Excel reader.
- Validación.
- Filesystem service.
- Batch model.
- Estados.
- Logging.

## Fase 2 — Outlook

Implementar:

- OutlookAdapter.
- MailSender.
- Worker.
- Queue.
- Manejo de errores.
- Cancelación.
- Progress events.

## Fase 3 — UI

Implementar:

- Layout.
- Tab Renombrar.
- Tab Enviar.
- Tab Historial.
- Progress.
- Preview.
- Error states.

## Fase 4 — Persistencia

Implementar:

- Configuración.
- Logs.
- Historial.
- Reintentos.

## Fase 5 — Hardening

Probar:

- 1 archivo.
- 10 archivos.
- 50 archivos.
- 100 archivos.
- 500 archivos.
- Archivo inexistente.
- Email inválido.
- Outlook cerrado.
- Outlook desconectado.
- Outlook bloqueado.
- Cancelación.
- Suspensión del equipo.
- Reinicio de la aplicación.
- Reintento.

## Fase 6 — Packaging

Generar:

```text
BoletaMailer.exe
```

y probar en un equipo Windows limpio.

---

# 29. Riesgos técnicos principales

## Riesgo 1 — Outlook COM

Es el mayor riesgo técnico del MVP.

Mitigación:

- Aislarlo mediante `OutlookAdapter`.
- Usar un único worker.
- Probar exhaustivamente.
- No acoplar el dominio a COM.

## Riesgo 2 — Nuevo Outlook

El MVP debe especificar explícitamente:

> Requiere Outlook Classic para Windows.

Si el entorno objetivo utiliza New Outlook exclusivamente, evaluar Microsoft Graph antes de continuar con COM.

## Riesgo 3 — Bloqueo de Outlook

Mitigación:

- Worker único.
- Timeout.
- Detección de errores.
- Recuperación controlada.
- No bloquear la UI.

## Riesgo 4 — Archivos incorrectos

Mitigación:

- Validación completa.
- Preview.
- Modo simulación.
- Confirmación antes del envío.

## Riesgo 5 — Suspensión del equipo

Mitigación:

- `SetThreadExecutionState`.
- Restauración garantizada del estado.
- Prueba con política de energía real.

---

# 30. Principios de diseño

1. **Desktop-first.**
2. **Self-contained.**
3. **Offline-first salvo la conexión normal de Outlook/Microsoft 365.**
4. **La UI nunca ejecuta directamente operaciones largas.**
5. **El dominio no conoce infraestructura.**
6. **Outlook es un adapter, no el dominio.**
7. **Validar antes de enviar.**
8. **No perder trazabilidad de cada documento.**
9. **Los errores individuales no deben destruir el lote completo.**
10. **No introducir infraestructura cloud innecesaria.**
11. **Preferir simplicidad para el MVP.**
12. **Diseñar puntos de extensión sin implementar complejidad prematuramente.**

---

# 31. Definición de terminado del MVP

El MVP estará terminado cuando un usuario pueda ejecutar:

```text
BoletaMailer.exe
      |
      v
Seleccionar Excel
      |
      v
Validar 50–500 registros
      |
      v
Preview
      |
      v
Confirmar
      |
      v
Enviar mediante Outlook Classic
      |
      v
Trabajar normalmente mientras continúa el proceso
      |
      v
Ver progreso
      |
      v
Finalizar
      |
      v
Consultar historial
      |
      v
Reintentar errores
```

sin necesitar instalar un runtime de desarrollo y sin que la aplicación requiera una infraestructura backend o cloud.

---

# 32. Primera tarea de implementación

Antes de construir la aplicación completa, realizar un **Technical Spike** que responda inequívocamente estas preguntas:

1. ¿Wails puede generar el ejecutable autocontenido esperado para el entorno Windows objetivo?
2. ¿Qué mecanismo de COM se utilizará desde Go para Outlook Classic?
3. ¿Se puede conectar a Outlook Classic desde una aplicación Go compilada?
4. ¿Se puede crear un MailItem?
5. ¿Se puede adjuntar un PDF?
6. ¿Se puede enviar correctamente?
7. ¿Qué sucede si Outlook está cerrado?
8. ¿Qué sucede si Outlook está ocupado?
9. ¿Cómo se detectan errores COM?
10. ¿`SetThreadExecutionState` funciona como esperado?
11. ¿El ejecutable funciona en una máquina limpia?
12. ¿El tamaño final del `.exe` es aceptable?

**No avanzar a una UI completa hasta validar Outlook + Go + Wails + packaging.**

---

# 33. Resultado esperado de la primera iteración

El primer entregable técnico no será todavía el MVP completo.

Debe ser:

```text
BoletaMailer.exe

Funciones:
    ✓ Arranca
    ✓ Muestra una UI mínima
    ✓ Lee un XLSX
    ✓ Detecta Outlook Classic
    ✓ Crea un correo
    ✓ Adjunta un PDF
    ✓ Envía un correo de prueba
    ✓ Ejecuta la operación fuera del hilo de UI
    ✓ Mantiene Windows despierto durante la prueba
    ✓ Compila como ejecutable autocontenido
```

Una vez validado esto, se puede construir encima el procesamiento de lotes y la UI definitiva.
