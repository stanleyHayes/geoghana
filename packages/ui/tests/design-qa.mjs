import { AxeBuilder } from "@axe-core/playwright";
import { chromium } from "playwright";

const surfaces = (process.env.GHANAGEO_QA_SURFACES ?? "web=http://localhost:3100,sandbox=http://localhost:3101,portal=http://localhost:3102,admin=http://localhost:3103")
  .split(",").map((entry) => { const [name, url] = entry.split("="); return { name, url }; });
const materialCatalog = ["neu", "glass", "clay"];
const themeCatalog = [{ name: "light", mode: "light", hue: 168 }, { name: "dark", mode: "dark", hue: 168 }, { name: "custom", mode: "light", hue: 232 }];
const viewportCatalog = [{ width: 320, height: 720 }, { width: 390, height: 844 }, { width: 768, height: 1024 }, { width: 1280, height: 800 }, { width: 1920, height: 1080 }];
const materials = process.env.GHANAGEO_QA_MATERIALS ? materialCatalog.filter((item) => process.env.GHANAGEO_QA_MATERIALS.split(",").includes(item)) : materialCatalog;
const themes = process.env.GHANAGEO_QA_THEMES ? themeCatalog.filter((item) => process.env.GHANAGEO_QA_THEMES.split(",").includes(item.name)) : themeCatalog;
const viewports = process.env.GHANAGEO_QA_VIEWPORTS ? viewportCatalog.filter((item) => process.env.GHANAGEO_QA_VIEWPORTS.split(",").includes(String(item.width))) : viewportCatalog;
const browser = await chromium.launch({ headless: true });
const failures = [];
let checks = 0;

try {
  for (const surface of surfaces) for (const material of materials) for (const theme of themes) for (const viewport of viewports) {
    const context = await browser.newContext({ viewport, reducedMotion: "reduce", colorScheme: theme.mode });
    await context.addInitScript(({ material, mode, hue }) => {
      localStorage.setItem("ghanageo-theme", JSON.stringify({ material, mode, brandH: hue, brandC: 0.115, version: 1 }));
    }, { material, mode: theme.mode, hue: theme.hue });
    const page = await context.newPage();
    const errors = [];
    page.on("console", (message) => { if (message.type() === "error") errors.push(message.text()); });
    page.on("pageerror", (error) => errors.push(error.message));
    const identity = `${surface.name}/${material}/${theme.name}/${viewport.width}`;
    try {
      const response = await page.goto(surface.url, { waitUntil: "domcontentloaded", timeout: 15_000 });
      if (!response?.ok()) failures.push(`${identity}: HTTP ${response?.status() ?? "no response"}`);
      const state = await page.evaluate(() => ({ overflow: document.documentElement.scrollWidth - innerWidth, material: document.documentElement.dataset.material, mode: document.documentElement.dataset.mode, main: Boolean(document.querySelector("main")), h1: Boolean(document.querySelector("h1")) }));
      if (state.overflow > 0) failures.push(`${identity}: horizontal overflow ${state.overflow}px`);
      if (state.material !== material || state.mode !== theme.mode) failures.push(`${identity}: theme did not resolve (${state.material}/${state.mode})`);
      if (!state.main || !state.h1) failures.push(`${identity}: missing main landmark or h1`);
      const axe = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21aa", "wcag22aa"]).analyze();
      for (const violation of axe.violations) {
        failures.push(`${identity}: axe ${violation.id} (${violation.nodes.length} node${violation.nodes.length === 1 ? "" : "s"})`);
        if (process.env.GHANAGEO_QA_VERBOSE === "1") for (const node of violation.nodes.slice(0, 8)) failures.push(`${identity}:   ${node.target.join(" ")} — ${node.failureSummary?.replace(/\s+/g, " ").trim()}`);
      }
      for (const error of errors) failures.push(`${identity}: console ${error}`);
      checks += 1;
    } catch (error) { failures.push(`${identity}: ${error instanceof Error ? error.message : String(error)}`); }
    finally { await context.close(); }
  }
} finally { await browser.close(); }

console.log(`Design QA: ${checks} matrix checks across ${surfaces.length} surfaces.`);
if (failures.length) { console.error(failures.join("\n")); process.exit(1); }
console.log("Design QA passed: zero axe violations, console errors, theme-resolution failures or body overflow.");
