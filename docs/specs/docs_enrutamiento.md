
# SPEC — Resolución de documentos y tabla de enrutamiento

**Proyecto:** BoletaMailer
**Tipo:** Decisión de arquitectura
**Estado:** Aprobado, con enmiendas (ver §0)
**Fecha:** 2026-08-26 (enmendado el mismo día tras revisión técnica)

---

## 0. Enmiendas a la versión original de este documento

Tras revisión, se aplican tres correcciones antes de adoptar este spec. El resto del
documento (contexto, regla de asociación DNI→archivo, preview antes de enviar,
justificación de separar Excel de rutas físicas) se mantiene sin cambios y sigue
siendo válido.

### 0.1 Se descarta Parquet como formato del routing dataset

Parquet es un formato columnar diseñado para lectura selectiva de columnas sobre
volúmenes grandes (millones de filas). Este proceso maneja 50–500 filas, leídas
completas (todas las columnas) una sola vez por ejecución — el caso de uso exactamente
opuesto al que Parquet optimiza. Adoptarlo aquí añade una dependencia Go no estándar
(binding a Apache Arrow), incrementa el tamaño del `.exe`, y produce un artefacto que
no puede inspeccionarse con un editor de texto — sin ganar nada a cambio en este orden
de magnitud.

En todo el documento, donde se lea `routing.parquet`, debe entenderse como:

```text
[]domain.BatchItem   — slice de Go en memoria, mientras el proceso corre
```

No se materializa como archivo intermedio en disco. Ver §0.2 para qué sí persiste.

### 0.2 Se descarta el historial de lotes en disco

El contexto original del MVP (`docs/boleta_mailer_mvp_context.md` §17) definía
persistencia de historial en `%LOCALAPPDATA%\BoletaMailer\history\*.json`. Se descarta:
el usuario exporta manualmente el resultado del envío a Excel (ver
`docs/specs/ui_spec_mvp.md` §3, botón "Generar reporte Excel"), y esa exportación es la
única copia que sobrevive al cierre de la app o al inicio de un nuevo lote. Si el
usuario cierra sin exportar, el resultado del lote se pierde — tradeoff aceptado a
cambio de simplicidad y cero acumulación de archivos.

Los **logs técnicos** (`%LOCALAPPDATA%\BoletaMailer\logs\`) sí se mantienen, pero como
un único archivo que se sobrescribe en cada arranque de la aplicación (no rotación
diaria como planteaba el contexto original) — evita acumular logs indefinidamente,
a costa de perder el log de una sesión anterior si el usuario ya cerró y reabrió la app.

### 0.3 Los estados usan el enum ya definido en `app/domain/batch_item.go`

Este documento originalmente definía sus propios 5 estados en español
(`PENDIENTE, VALIDADO, ENVIANDO, ENVIADO, ERROR`). Se reemplazan por el
`BatchItemStatus` ya implementado en Go — 8 estados, en inglés:

```text
PENDING, VALIDATING, INVALID, READY, SENDING, SENT, FAILED, CANCELLED
```

Motivo: los identificadores de código siguen la convención idiomática de Go (inglés,
sin colisión con el tipo `error` del lenguaje — un estado llamado `ERROR` es ambiguo al
lado de `err.Error()`). Además esta versión es más granular: distingue `INVALID`
(el registro no puede resolverse — DNI sin documento, correo mal formado — nunca llega
a intentar el envío) de `FAILED` (el envío se intentó y Outlook/COM falló), y agrega
`CANCELLED`, requerido por el contexto original §22 para cancelación cooperativa.

El texto visible al usuario en la UI (React) sí se traduce libremente al español; lo
que cambia aquí son solo los identificadores internos.

Mapeo respecto a la versión original de este documento:

| Original (espl, 5 estados) | Reemplazado por |
|---|---|
| `PENDIENTE` | `StatusPending` |
| `VALIDADO` | `StatusReady` |
| `ENVIANDO` | `StatusSending` |
| `ENVIADO` | `StatusSent` |
| `ERROR` | `StatusInvalid` (falla de datos/asociación) o `StatusFailed` (falla de envío) — ver distinción en §0.3 arriba |
| *(no existía)* | `StatusValidating`, `StatusCancelled` |

---

## 1. Contexto

El Mail Sender debe permitir el envío masivo de documentos personalizados a una lista de destinatarios.

La información de los destinatarios será proporcionada mediante un archivo Excel con una estructura mínima:

| DNI      | NOMBRE      | CORREO                                 |
| -------- | ----------- | -------------------------------------- |
| 12345678 | Juan Pérez | [juan@gmail.com](mailto:juan@gmail.com) |
| 87654321 | Ana López  | [ana@gmail.com](mailto:ana@gmail.com)   |

Los documentos, por ejemplo boletas de pago, estarán almacenados en una carpeta seleccionada por el usuario y previamente renombrados utilizando una convención basada en el DNI:

```text
boletas/
├── 12345678.pdf
├── 87654321.pdf
├── 45678912.pdf
└── ...
```

Se evaluaron dos alternativas para relacionar los destinatarios con sus documentos:

1. Incluir la ruta física del documento dentro del Excel.
2. Permitir que el usuario seleccione el Excel y la carpeta de documentos, y que la aplicación construya internamente la relación.

---

## 2. Decisión

Se utilizará la **segunda alternativa**.

El usuario proporcionará:

* Un Excel con la información de los destinatarios.
* Una carpeta que contiene los documentos.

La aplicación será responsable de resolver la correspondencia entre cada registro del Excel y su documento utilizando una regla de identificación definida por el negocio.

Para el MVP, la regla será:

```text
DNI del destinatario = nombre base del archivo
```

Ejemplo:

```text
DNI:          12345678
Documento:    12345678.pdf
```

La aplicación generará una **tabla de enrutamiento interna**, que será materializada en formato Parquet.

---

## 3. Principio de diseño

El Excel representa **información de negocio**, mientras que la ubicación física de los archivos representa un **detalle de infraestructura**.

Por lo tanto:

```text
Excel
  │
  │ Información de negocio
  ▼
Destinatarios
  │
  │
  ├───────────────┐
  │               │
  ▼               ▼
Carpeta       Regla de asociación
documentos          │
  │                 │
  └────────┬────────┘
           ▼
     Routing Dataset
      routing.parquet
```

El Excel no deberá contener rutas físicas de archivos.

---

# 4. Entradas del proceso

## 4.1 Lista de destinatarios

El usuario deberá proporcionar un archivo Excel con al menos las siguientes columnas:

| Campo  | Tipo   | Obligatorio | Descripción                          |
| ------ | ------ | ----------: | ------------------------------------- |
| DNI    | string |         Sí | Identificador único del destinatario |
| NOMBRE | string |         Sí | Nombre del destinatario               |
| CORREO | string |         Sí | Dirección de correo electrónico     |

La aplicación deberá validar:

* Existencia de las columnas requeridas.
* DNI no vacío.
* Correo no vacío.
* Formato válido del correo.
* Ausencia de DNI duplicados.
* Ausencia de registros completamente vacíos.

---

## 4.2 Carpeta de documentos

El usuario deberá seleccionar una carpeta que contenga los documentos que serán enviados.

Ejemplo:

```text
C:\Procesos\Boletas\
```

Contenido:

```text
C:\Procesos\Boletas\
├── 12345678.pdf
├── 87654321.pdf
├── 45678912.pdf
└── 98765432.pdf
```

La aplicación deberá descubrir los archivos disponibles dentro de la carpeta.

La ruta absoluta podrá almacenarse internamente, pero no deberá formar parte del Excel de entrada.

---

# 5. Regla de asociación

La primera versión del sistema utilizará una asociación basada en el DNI.

```text
DNI del Excel
       │
       ▼
nombre base del archivo
       │
       ▼
extensión
```

Ejemplo:

```text
Excel:

DNI = 12345678

        ↓

Archivo esperado:

12345678.pdf
```

La extensión del archivo podrá ser parametrizable, pero el identificador principal deberá mantenerse independiente de la extensión.

Conceptualmente:

```python
document_id = file.stem
```

Por lo tanto:

```text
12345678.pdf → 12345678
12345678.xml → 12345678
12345678.docx → 12345678
```

La implementación concreta de múltiples documentos por DNI será definida en una especificación posterior si el negocio lo requiere.

---

# 6. Tabla de enrutamiento

Una vez procesadas las entradas, el sistema deberá construir, en memoria, un slice de
Go que representa la relación entre destinatario, correo, documento y estado del
procesamiento:

```go
type BatchItem struct {
    Employee        Employee // Name, DNI, Email
    Document        Document // Path
    Status          BatchItemStatus
    ValidationError string
    SendError       string
}
```

(tipos ya definidos en `app/domain/`). No se materializa como archivo intermedio en
disco — ver §0.1.

Campos equivalentes a la tabla conceptual original:

| Campo conceptual | Campo real (`domain.BatchItem`) |
| ------------- | --------------------------------- |
| dni, nombre   | `Employee.DNI`, `Employee.Name`   |
| correo        | `Employee.Email`                  |
| archivo, ruta | `Document.Path`                   |
| estado        | `Status` (`BatchItemStatus`)      |
| error         | `ValidationError` o `SendError`, según en qué etapa ocurrió |

No se incluye `fecha_proceso` por registro — el timestamp relevante es el del batch
completo (`StartedAt`/`FinishedAt` en el agregado `Batch`), no por línea; ver §0.2 sobre
por qué no se persiste un dataset histórico por ejecución.

---

# 7. Estados del routing dataset

El estado de cada `BatchItem` usa el enum ya implementado en
`app/domain/batch_item.go` — ver la enmienda en §0.3 para el razonamiento completo.

```text
PENDING → VALIDATING → INVALID
                     └→ READY → SENDING → SENT
                                        └→ FAILED
(CANCELLED alcanzable desde VALIDATING, READY o SENDING)
```

Esto permite separar claramente tres momentos distintos, no dos:

1. **Resolución/validación de datos** (`INVALID`) — el registro nunca podrá enviarse
   tal como está: DNI sin documento, correo mal formado, DNI duplicado. Se detecta
   antes de tocar Outlook.
2. **Envío** (`SENDING` → `SENT`/`FAILED`) — el registro era válido, pero Outlook/COM
   falló al momento de enviar (Outlook cerrado, timeout, error de red).
3. **Cancelación** (`CANCELLED`) — el usuario detuvo el lote; ver contexto original §22.

Distinguir (1) de (2) importa porque son causas y remedios distintos: un `INVALID` se
arregla corrigiendo el Excel o la carpeta de documentos, antes de reintentar; un
`FAILED` se arregla con el botón "Reintentar fallidos" sin tocar los datos de entrada.

---

# 8. Validaciones de asociación

Antes de permitir el envío, la aplicación deberá verificar que cada registro pueda ser
resuelto. Un registro que falla esta etapa queda en `StatusInvalid`, no en un genérico
"ERROR" — ver §7.

### Caso válido

```text
DNI Excel:       12345678
Archivo:         12345678.pdf
Resultado:       READY
```

### Documento inexistente

```text
DNI Excel:       12345678
Archivo esperado: 12345678.pdf
Resultado:        INVALID
```

Error:

```text
Documento no encontrado
```

### DNI duplicado

```text
12345678
12345678
```

Resultado:

```text
INVALID
DNI duplicado en lista de destinatarios
```

### Archivo ambiguo

Si existen múltiples documentos para un mismo DNI y la configuración no define cómo resolverlos:

```text
12345678.pdf
12345678_2.pdf
```

el registro deberá quedar en:

```text
INVALID
```

y el sistema no deberá enviar automáticamente el documento.

---

# 9. Preview antes del envío

La construcción de la tabla de enrutamiento (§6) deberá ocurrir **antes del envío de
correos**.

El flujo será:

```text
1. Cargar Excel
       ↓
2. Seleccionar carpeta
       ↓
3. Leer documentos
       ↓
4. Resolver asociaciones
       ↓
5. Construir []domain.BatchItem en memoria
       ↓
6. Ejecutar validaciones
       ↓
7. Mostrar preview
       ↓
8. Usuario confirma
       ↓
9. Ejecutar envíos
```

El usuario deberá poder identificar previamente:

* Cantidad de registros.
* Cantidad de documentos encontrados.
* Cantidad de documentos faltantes.
* Cantidad de registros inválidos.
* Cantidad de registros listos para enviar.

Ejemplo:

```text
──────────────────────────────────────
VALIDACIÓN
──────────────────────────────────────

Destinatarios:       1,250
Documentos:          1,245

Listos para enviar:  1,240
Documentos faltantes:    5
Errores de datos:        5

                 [ Cancelar ] [ Continuar ]
──────────────────────────────────────
```

---

# 10. Justificación de la decisión

## 10.1 Evita acoplamiento con el sistema de archivos

No se recomienda almacenar:

```text
C:\Users\Ricardo\Documents\Boletas\12345678.pdf
```

dentro del Excel.

La ruta depende de:

* computadora;
* usuario;
* estructura de carpetas;
* unidad utilizada;
* entorno de ejecución.

La información de negocio no debería depender de estos detalles.

---

## 10.2 Permite reutilizar el Excel

El mismo Excel puede utilizarse con diferentes carpetas:

```text
empleados.xlsx
       │
       ├── Agosto/
       ├── Septiembre/
       └── Octubre/
```

sin modificar la lista de destinatarios.

---

## 10.3 Centraliza la lógica de asociación

La relación:

```text
DNI → documento
```

queda bajo responsabilidad de la aplicación.

Esto permite evolucionar posteriormente la regla sin modificar el formato de entrada.

Por ejemplo:

```text
MVP:
DNI → archivo

Futuro:
DNI + periodo → archivo

Futuro:
DNI + tipo_documento + periodo → archivo
```

---

## 10.4 Permite trazabilidad

La tabla de enrutamiento en memoria (`[]domain.BatchItem`, ver §6) funciona como
dataset operativo del proceso mientras dura la ejecución, y se exporta a Excel al
finalizar para que el usuario conserve un registro (ver §0.2).

Permite conocer:

```text
¿Qué documento corresponde?
¿Fue encontrado?
¿Fue validado?
¿Fue enviado?
¿Falló?
¿Por qué falló?
```

Esto será especialmente importante para procesos masivos.

---

# 11. Responsabilidades

### Excel

Responsable de:

```text
Identidad del destinatario
Nombre
Correo
```

### Carpeta

Responsable de:

```text
Documentos disponibles
```

### Motor de resolución

Responsable de:

```text
Asociar destinatarios con documentos
```

### Tabla de enrutamiento (`[]domain.BatchItem`, en memoria)

Responsable de:

```text
Representar el estado y resultado del proceso
```

### Mail Sender

Responsable de:

```text
Enviar los documentos asociados
```

---

# 12. Estructura conceptual del MVP

```text
                  ┌──────────────────┐
                  │   Excel Input    │
                  │                  │
                  │ DNI              │
                  │ NOMBRE           │
                  │ CORREO           │
                  └────────┬─────────┘
                           │
                           │
                           ▼
                  ┌──────────────────┐
                  │ Document Resolver│
                  └────────┬─────────┘
                           ▲
                           │
                  ┌────────┴─────────┐
                  │                  │
          ┌───────┴──────┐   ┌───────┴───────┐
          │ Documents    │   │ Association   │
          │ Folder       │   │ Rule          │
          └──────────────┘   └───────────────┘
                           │
                           ▼
                  ┌──────────────────┐
                  │ []BatchItem      │
                  │ (en memoria)     │
                  │                  │
                  │ DNI              │
                  │ Correo           │
                  │ Documento        │
                  │ Ruta             │
                  │ Estado           │
                  │ Error            │
                  └────────┬─────────┘
                           │
                           ▼
                  ┌──────────────────┐
                  │ Mail Sender      │
                  └────────┬─────────┘
                           │
                           ▼
                  ┌──────────────────┐
                  │ Export a Excel   │
                  │ (bajo demanda)   │
                  └──────────────────┘
```

---

# 13. Decisión final

**El Excel no almacenará rutas físicas de documentos.**

El usuario seleccionará:

```text
[ Excel de destinatarios ]
          +
[ Carpeta de documentos ]
```

La aplicación resolverá la asociación mediante una regla basada inicialmente en el DNI y construirá, en memoria, una tabla de enrutamiento (`[]domain.BatchItem`) que actuará como **dataset operativo de enrutamiento y seguimiento del proceso** mientras dura la ejecución — exportable a Excel bajo demanda del usuario, sin persistirse en disco como historial entre ejecuciones (ver §0).

Esta decisión mantiene separado el **dato de negocio** de los **detalles de infraestructura local**, facilita la reutilización del Excel y deja preparada la arquitectura para incorporar posteriormente reglas de asociación más complejas.

---

## 14. Fuera de alcance del MVP

No forman parte de esta decisión:

* Almacenamiento de documentos en S3.
* Asociación mediante OCR.
* Asociación mediante contenido del documento.
* Múltiples documentos por DNI.
* Reglas complejas por periodo o tipo de documento.
* Base de datos transaccional.
* Persistencia histórica de múltiples ejecuciones.

Estas capacidades podrán evaluarse en futuras iteraciones.
