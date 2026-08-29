import { defineConfig } from "tsup";

export default defineConfig([
  {
    entry: ["src/index.ts"],
    format: ["esm"],
    dts: true,
    clean: true,
    noExternal: ["@ghanageo/client", "@ghanageo/core", "@ghanageo/data"],
    outExtension: () => ({ js: ".mjs" }),
  },
  {
    entry: ["src/index.ts"],
    format: ["cjs"],
    clean: false,
    noExternal: ["@ghanageo/client", "@ghanageo/core", "@ghanageo/data"],
    outExtension: () => ({ js: ".cjs" }),
  },
]);
