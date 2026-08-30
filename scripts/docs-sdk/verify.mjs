#!/usr/bin/env node
import { readFileSync } from "node:fs";
import { spawnSync } from "node:child_process";
import { resolve } from "node:path";
import { examples } from "./manifest.mjs";

const root = resolve(import.meta.dirname, "../..");
const forbidden = [
  /(?:api[_-]?key|authorization)\s*[:=]\s*["'][A-Za-z0-9_\-]{16,}/i,
  /(?:ghp|github_pat|sk_live|sk_test)_[A-Za-z0-9_\-]{12,}/,
];

for (const example of examples) {
  const source = readFileSync(resolve(root, example.source), "utf8");
  for (const pattern of forbidden) {
    if (pattern.test(source)) throw new Error(`${example.source} contains a secret-shaped value`);
  }
}

const checks = [
  ["generated docs drift", "node", ["scripts/docs-sdk/generate.mjs", "--check"]],
  ["TypeScript example", "pnpm", ["exec", "tsc", "-p", "scripts/docs-sdk/tsconfig.json"]],
  ["React example", "pnpm", ["--filter", "@ghanageo/react", "typecheck:examples"]],
  ["Python examples", "python3", ["-m", "compileall", "-q", "sdks/python/examples"]],
  ["Go example", "go", ["test", "./..."], "sdks/go"],
  ["Dart example", "dart", ["analyze", "example/regions.dart"], "sdks/dart"],
  ["Java example", "./mvnw", ["-q", "-pl", "examples", "-am", "-DskipTests", "compile"], "sdks/java"],
  [".NET example", "dotnet", ["build", "examples/ConsoleExample/ConsoleExample.csproj", "--no-restore", "--nologo"], "sdks/dotnet"],
  ["PHP example", "php", ["-l", "examples/regions.php"], "sdks/php"],
  ["curl example", "sh", ["-n", "scripts/docs-sdk/examples/curl.sh"]],
  ["web docs", "pnpm", ["--filter", "@ghanageo/web", "build"]],
  ["links", "node", ["scripts/check-links.mjs"]],
];

for (const [label, command, args, cwd = "."] of checks) {
  console.log(`\n[docs-sdk] ${label}`);
  const env = label === "Go example" ? { ...process.env, GOWORK: "off" } : process.env;
  const result = spawnSync(command, args, { cwd: resolve(root, cwd), stdio: "inherit", env });
  if (result.error?.code === "ENOENT") {
    console.error(`Required tool is unavailable: ${command}`);
    process.exit(1);
  }
  if (result.status !== 0) process.exit(result.status ?? 1);
}

console.log(`\nVerified ${examples.length} extracted SDK examples, documentation drift, links, and secret hygiene.`);
