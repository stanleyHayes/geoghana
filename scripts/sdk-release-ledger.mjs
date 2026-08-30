#!/usr/bin/env node
import { createHash } from "node:crypto";
import { existsSync, readFileSync, writeFileSync } from "node:fs";

const [command, path, manifestPath, checkpoint, evidencePath] = process.argv.slice(2);
if (!command || !path) throw new Error("usage: sdk-release-ledger.mjs <init|has|complete|evidence> <ledger> [manifest-or-checkpoint] [checkpoint] [evidence.json]");
const read = () => JSON.parse(readFileSync(path, "utf8"));
const canonical = (value) => Array.isArray(value) ? `[${value.map(canonical).join(",")}]` : value && typeof value === "object" ? `{${Object.keys(value).sort().map((key) => `${JSON.stringify(key)}:${canonical(value[key])}`).join(",")}}` : JSON.stringify(value);
if (command === "init") {
  const manifestBytes = readFileSync(manifestPath);
  const manifest = JSON.parse(manifestBytes);
  const binding = { version: manifest.release.version, commit: manifest.release.commit, manifestSha256: createHash("sha256").update(manifestBytes).digest("hex") };
  if (existsSync(path)) {
    const ledger = read();
    for (const [key, value] of Object.entries(binding)) if (ledger[key] !== value) throw new Error(`release ledger ${key} mismatch`);
  } else writeFileSync(path, `${JSON.stringify({ schemaVersion: 2, ...binding, completed: {} }, null, 2)}\n`);
} else if (command === "has") {
  const completed = read().completed;
  process.exit((Array.isArray(completed) ? completed.includes(manifestPath) : Object.hasOwn(completed, manifestPath)) ? 0 : 1);
} else if (command === "complete") {
  const ledger = read();
  if (Array.isArray(ledger.completed)) ledger.completed = Object.fromEntries(ledger.completed.map((name) => [name, { legacy: true }]));
  const evidence = evidencePath ? JSON.parse(readFileSync(evidencePath, "utf8")) : {};
  if (!evidence || typeof evidence !== "object" || Array.isArray(evidence)) throw new Error("checkpoint evidence must be a JSON object");
  if (ledger.completed[checkpoint] && canonical(ledger.completed[checkpoint]) !== canonical(evidence)) throw new Error(`checkpoint ${checkpoint} evidence mismatch`);
  ledger.completed[checkpoint] = evidence;
  writeFileSync(path, `${JSON.stringify(ledger, null, 2)}\n`);
} else if (command === "evidence") {
  const completed = read().completed;
  const evidence = Array.isArray(completed) ? undefined : completed[manifestPath];
  if (!evidence) process.exit(1);
  process.stdout.write(`${JSON.stringify(evidence)}\n`);
} else throw new Error(`unknown ledger command: ${command}`);
