import { readFileSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";

const [directory, version] = process.argv.slice(2);
if (!directory || !/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-(?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*))*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/.test(version ?? "")) {
  throw new Error("usage: stamp-staged-sdk-package.mjs DIRECTORY VERSION");
}
const manifestPath = resolve(directory, "package.json");
const manifest = JSON.parse(readFileSync(manifestPath, "utf8"));
manifest.version = version;
for (const field of ["dependencies", "optionalDependencies", "peerDependencies"]) {
  for (const dependency of Object.keys(manifest[field] ?? {})) {
    if (dependency === "ghanageo" || dependency.startsWith("@ghanageo/")) manifest[field][dependency] = version;
  }
}
writeFileSync(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`);
