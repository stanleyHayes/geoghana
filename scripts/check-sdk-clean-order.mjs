import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const verifier = readFileSync(resolve(import.meta.dirname, "verify-sdk-release.sh"), "utf8");
assert.match(
  verifier,
  /packages=\(core client react node data proto\)/,
  "SDK packages must remain in dependency order so clean declaration outputs exist before consumers build",
);
const loopStart = verifier.indexOf('for package_name in "${packages[@]}"');
const loopEnd = verifier.indexOf("done", loopStart);
const loop = verifier.slice(loopStart, loopEnd);
const build = loop.indexOf('pnpm --dir "$repo_root/packages/$package_name" build');
const test = loop.indexOf('pnpm --dir "$repo_root/packages/$package_name" test');
assert.ok(build >= 0 && test > build, "each package must build before its tests run on a clean checkout");
const cliBuild = verifier.indexOf('pnpm --dir "$repo_root/packages/cli" build');
const cliTest = verifier.indexOf('pnpm --dir "$repo_root/packages/cli" test');
assert.ok(cliBuild >= 0 && cliTest > cliBuild, "umbrella package must build before its tests");
console.log("verified dependency-ordered clean build-before-test release sequence");
