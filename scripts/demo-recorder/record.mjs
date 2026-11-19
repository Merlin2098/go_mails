// Graba la demo completa de Despacho contra la app REAL (backend Go en modo
// demo), no contra un mock:
//
//   1. Genera datos ficticios frescos (go run ./cmd/demodata).
//   2. Compila el frontend y un binario Wails en modo dev, que expone la app
//      con sus bindings de Go en http://localhost:34115.
//   3. Playwright recorre los 5 pasos, toma screenshots y graba video.
//   4. ffmpeg convierte el .webm de Playwright a MP4 (H.264) para redes.
//
// Lo único simulado son los selectores nativos de archivo/carpeta de Windows
// (no existen en un navegador): se reemplazan por rutas fijas. Todo lo demás
// (lectura del Excel, renombrado, validación, envío, .eml) lo hace Go.
//
//   cd scripts/demo-recorder && npm install && npm run record
//   node record.mjs --skip-build     # reutiliza el binario ya compilado
//   node record.mjs --headed         # ver el navegador mientras graba

import { spawn, spawnSync } from "node:child_process";
import { existsSync, mkdirSync, readdirSync, renameSync, rmSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright";

// ffmpeg con libx264, en este orden: $FFMPEG_PATH, el binario de
// ffmpeg-static (si su script de instalación pudo descargarlo) o el del PATH.
async function findFfmpeg() {
  if (process.env.FFMPEG_PATH && existsSync(process.env.FFMPEG_PATH)) return process.env.FFMPEG_PATH;
  try {
    const p = (await import("ffmpeg-static")).default;
    if (p && existsSync(p)) return p;
  } catch {
    /* paquete opcional */
  }
  const where = spawnSync(process.platform === "win32" ? "where" : "which", ["ffmpeg"], { encoding: "utf8" });
  if (where.status === 0) return where.stdout.split(/\r?\n/)[0].trim();
  // winget lo deja aquí; una terminal abierta antes de instalarlo no lo ve en el PATH.
  const winget = join(process.env.LOCALAPPDATA ?? "", "Microsoft", "WinGet", "Links", "ffmpeg.exe");
  return existsSync(winget) ? winget : null;
}

const ROOT = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..");
const OUT = join(ROOT, "docs", "demo");
const SHOTS = join(OUT, "screenshots");
const DATA = join(ROOT, "demo");
const EML = join(DATA, "salida_eml");
const DEV_BIN = join(ROOT, "build", "bin", process.platform === "win32" ? "Despacho-dev.exe" : "Despacho-dev");
const URL = "http://localhost:34115";
const SIZE = { width: 1280, height: 800 };

const args = new Set(process.argv.slice(2));
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

function run(cmd, cmdArgs, cwd = ROOT) {
  console.log(`$ ${cmd} ${cmdArgs.join(" ")}`);
  // Solo npm necesita shell en Windows (es un .cmd). Con shell, las rutas con
  // espacios ("VS Code") se partirían, así que nunca se le pasan rutas.
  const shell = process.platform === "win32" && cmd === "npm";
  const res = spawnSync(cmd, cmdArgs, { cwd, stdio: "inherit", shell });
  if (res.status !== 0) throw new Error(`${cmd} falló (código ${res.status})`);
}

async function waitForServer(url, timeoutMs = 30000) {
  const start = Date.now();
  while (Date.now() - start < timeoutMs) {
    try {
      if ((await fetch(url)).ok) return;
    } catch {
      /* todavía no arranca */
    }
    await sleep(300);
  }
  throw new Error(`La app no respondió en ${url}`);
}

// --- 1. datos y build -------------------------------------------------------
rmSync(DATA, { recursive: true, force: true });
run("go", ["run", "./cmd/demodata", "-out", DATA, "-count", "12"]);
if (!args.has("--skip-build") || !existsSync(DEV_BIN)) {
  run("npm", ["run", "build"], join(ROOT, "frontend"));
  run("go", ["build", "-tags", "dev", "-o", DEV_BIN, "."]);
}

// --- 2. app en modo dev -----------------------------------------------------
const app = spawn(DEV_BIN, ["-assetdir", join(ROOT, "frontend", "dist"), "-devserver", "localhost:34115", "-loglevel", "Warning"], {
  cwd: ROOT,
  stdio: "ignore",
});
const stopApp = () => {
  if (!app.killed) app.kill();
};
process.on("exit", stopApp);

try {
  await waitForServer(URL);
  await record();
} finally {
  stopApp();
}

// --- 3. grabación -----------------------------------------------------------
async function record() {
  rmSync(OUT, { recursive: true, force: true });
  mkdirSync(SHOTS, { recursive: true });
  const rawDir = join(OUT, ".raw");

  const browser = await chromium.launch({ headless: !args.has("--headed") });
  const context = await browser.newContext({
    viewport: SIZE,
    deviceScaleFactor: 1,
    recordVideo: { dir: rawDir, size: SIZE },
    locale: "es-ES",
  });
  // Cursor visible: los videos de Playwright no lo muestran, y en una demo
  // el espectador necesita ver dónde se hace clic.
  await context.addInitScript(() => {
    addEventListener("DOMContentLoaded", () => {
      const c = document.createElement("div");
      c.id = "demo-cursor";
      c.style.cssText =
        "position:fixed;left:0;top:0;width:22px;height:22px;margin:-11px 0 0 -11px;border-radius:50%;" +
        "background:rgba(242,194,48,.55);border:2px solid #1B1A17;z-index:2147483647;pointer-events:none;" +
        "transition:transform .12s ease;transform:translate(-100px,-100px)";
      document.body.appendChild(c);
      let x = -100, y = -100;
      addEventListener("mousemove", (e) => {
        x = e.clientX; y = e.clientY;
        c.style.transform = `translate(${x}px,${y}px)`;
      }, true);
      addEventListener("mousedown", () => (c.style.transform = `translate(${x}px,${y}px) scale(.7)`), true);
      addEventListener("mouseup", () => (c.style.transform = `translate(${x}px,${y}px)`), true);
    });
  });

  const page = await context.newPage();
  page.setDefaultTimeout(20000);
  await page.goto(URL);
  await page.waitForFunction(() => window.go?.desktop?.App && window.go?.desktop?.CampaignService);

  // Reemplaza SOLO los diálogos nativos de Windows por rutas fijas.
  await page.evaluate(
    ({ data, docs, eml }) => {
      window.go.desktop.App.SelectDataFile = async () => data;
      window.go.desktop.App.SelectFolder = async (title) => (String(title).includes(".eml") ? eml : docs);
    },
    { data: join(DATA, "plantilla_envio.xlsx"), docs: join(DATA, "documentos_origen"), eml: EML }
  );

  const shot = async (name) => {
    await sleep(350);
    // El cursor sirve en el video, pero en una imagen fija tapa el texto.
    await page.screenshot({ path: join(SHOTS, name), style: "#demo-cursor{display:none!important}" });
    console.log(`  screenshot ${name}`);
  };

  // Movimiento de mouse visible antes de cada clic.
  const click = async (locator) => {
    await locator.waitFor({ state: "visible" });
    await locator.scrollIntoViewIfNeeded();
    const box = await locator.boundingBox();
    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2, { steps: 18 });
    await sleep(180);
    await page.mouse.down();
    await sleep(70);
    await page.mouse.up();
    await sleep(450);
  };

  const typeInto = async (locator, text) => {
    await click(locator);
    await locator.press("Control+A");
    await locator.press("Backspace");
    await locator.pressSequentially(text, { delay: 45 });
    await sleep(500);
  };

  const btn = (name) => page.getByRole("button", { name, exact: false });

  // 01 · Conexión
  await sleep(900);
  await shot("01-conexion-inicio.png");
  await click(page.locator("button.prov", { hasText: "Modo demo" }));
  await click(btn("Elegir carpeta"));
  await click(btn("Usar modo demo"));
  await typeInto(page.locator("#delay"), "1");
  await page.locator("#delay").blur();
  await sleep(500);
  await shot("01-conexion-modo-demo.png");
  await click(btn("Siguiente: cargar datos"));

  // 02 · Datos
  await click(btn("Seleccionar Excel o CSV"));
  await page.getByText("plantilla_envio.xlsx").waitFor();
  await page.getByText("con errores").waitFor();
  await sleep(800);
  await shot("02-datos-validados.png");
  await click(btn("Siguiente: plantilla"));

  // 03 · Plantilla (asunto/cuerpo vienen de la hoja «Mensaje»)
  await page.locator(".envelope").waitFor();
  await sleep(700);
  await click(page.getByRole("button", { name: "Destinatario siguiente" }));
  await click(page.getByRole("button", { name: "Destinatario siguiente" }));
  await shot("03-plantilla-vista-previa.png");
  await click(btn("Siguiente: adjuntos"));

  // 04 · Adjuntos: renombrar y emparejar
  await click(btn("Elegir carpeta"));
  await typeInto(page.locator("#rule"), "CERT-{{documento_id}}_{{nombre}}{{ext}}");
  await btn("Renombrar 11 archivos").waitFor();
  await shot("04-renombrar-vista-previa.png");
  await click(btn("Renombrar 11 archivos"));
  await page.getByText("11 renombrados").waitFor();
  await typeInto(page.locator("#pattern"), "CERT-{{documento_id}}_*.pdf");
  await btn("Revisar y enviar 10 correos").waitFor();
  await sleep(600);
  await shot("04-adjuntos-emparejados.png");
  await click(btn("Revisar y enviar 10 correos"));

  // 05 · Envío
  await sleep(700);
  await shot("05-envio-listo.png");
  await click(btn("Enviar 10 correos"));
  await page.locator(".kpi b", { hasText: /^[4-7] \/ 10$/ }).waitFor({ timeout: 30000 });
  await shot("05-envio-en-curso.png");
  await page.getByRole("heading", { name: "Lote terminado" }).waitFor({ timeout: 60000 });
  await sleep(1200);
  await shot("05-envio-terminado.png");
  await sleep(1500);

  const video = page.video();
  await context.close();
  await browser.close();

  // --- 4. webm -> mp4 -------------------------------------------------------
  const webm = join(OUT, "despacho-demo.webm");
  renameSync(await video.path(), webm);
  rmSync(rawDir, { recursive: true, force: true });

  const mp4 = join(OUT, "despacho-demo.mp4");
  const ffmpeg = await findFfmpeg();
  if (ffmpeg) {
    run(ffmpeg, ["-y", "-loglevel", "error", "-i", webm, "-c:v", "libx264", "-preset", "slow", "-crf", "18",
      "-pix_fmt", "yuv420p", "-movflags", "+faststart", "-an", mp4]);
    rmSync(webm); // el MP4 es el entregable; el .webm era intermedio
  } else {
    console.warn("\n[aviso] No se encontró ffmpeg con H.264: queda solo el .webm.");
    console.warn("        Instala ffmpeg (winget install Gyan.FFmpeg) o define FFMPEG_PATH y vuelve a correr.");
  }

  const emls = existsSync(EML) ? readdirSync(EML).length : 0;
  console.log(`\nListo:\n  ${existsSync(mp4) ? mp4 : webm}\n  ${SHOTS} (${readdirSync(SHOTS).length} imágenes)\n  .eml generados por Go: ${emls}`);
}
