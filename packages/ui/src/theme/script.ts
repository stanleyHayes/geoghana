import { DEFAULT_THEME, STORAGE_KEY } from "./types";

/**
 * The blocking inline script that prevents a flash of the wrong theme.
 *
 * It MUST be synchronous and inline in <head>: a deferred or external script
 * runs after first paint, which is exactly the bug this avoids. Everything is
 * wrapped in try/catch because blocked or corrupted storage must never
 * white-screen the app. (DESIGN_SYSTEM.md 7.3)
 */
export function themeInitScript(): string {
  return `(function(){try{
var d=document.documentElement,s=localStorage.getItem(${JSON.stringify(STORAGE_KEY)}),t=s?JSON.parse(s):{};
var m=(t.mode&&t.mode!=='system')?t.mode:(matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light');
d.dataset.mode=m;
d.dataset.material=t.material||${JSON.stringify(DEFAULT_THEME.material)};
d.dataset.density=t.density||${JSON.stringify(DEFAULT_THEME.density)};
d.style.colorScheme=m;
if(t.brandH!=null)d.style.setProperty('--brand-h',String(t.brandH));
if(t.brandC!=null)d.style.setProperty('--brand-c',String(t.brandC));
}catch(e){}})();`;
}
