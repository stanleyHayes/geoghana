#!/usr/bin/env node

import { createHash } from "node:crypto";
import { readFile, stat } from "node:fs/promises";
import { resolve } from "node:path";

const [rootArg, expectedVersion, expectedCommit] = process.argv.slice(2);
if (!rootArg || !expectedVersion || !expectedCommit) throw new Error("usage: verify-sdk-release-bundle.mjs <root> <version> <commit>");
const root = resolve(rootArg);
const manifest = JSON.parse(await readFile(resolve(root, "compatibility-manifest.json"), "utf8"));
if (manifest.release?.version !== expectedVersion) throw new Error(`release version mismatch: ${manifest.release?.version}`);
if (manifest.release?.commit !== expectedCommit) throw new Error(`release commit mismatch: ${manifest.release?.commit}`);
if (manifest.publication?.attempted !== false) throw new Error("refusing a bundle already marked as published");
if (!Array.isArray(manifest.artifacts) || manifest.artifacts.length !== 16) throw new Error("expected exactly 16 release artifacts");
const families = ["typescript", "python", "go", "dart", "flutter", "java", "dotnet", "php"];
for (const family of families) {
  const sdk = manifest.sdks?.[family];
  if (!sdk || !sdk.apiTarget || !sdk.testedDataset || !sdk.install) throw new Error(`${family}: incomplete compatibility/install metadata`);
  if (family === "typescript" ? !sdk.packages || Object.keys(sdk.packages).length !== 7 : !sdk.codeVersion) throw new Error(`${family}: missing code version`);
}
if (/SNAPSHOT/i.test(manifest.sdks.java.codeVersion) || !String(manifest.sdks.java.install).includes(manifest.sdks.java.codeVersion)) throw new Error("Java staged version/install metadata is not publishable");
if (manifest.sdks.go.sourceRepository !== "github.com/ghanageo/ghanageo-go" || manifest.sdks.php.sourceRepository !== "github.com/ghanageo/ghanageo-php") throw new Error("source registry identity is not canonical");
if (!/^[a-f0-9]{64}$/.test(manifest.contracts?.aggregateSha256 ?? "") || !/^[a-f0-9]{64}$/.test(manifest.contracts?.conformanceSha256 ?? "")) throw new Error("invalid contract compatibility digests");
for (const artifact of manifest.artifacts) {
  if (!/^[a-f0-9]{64}$/.test(artifact.sha256)) throw new Error(`invalid digest for ${artifact.path}`);
  const path = resolve(root, artifact.path);
  if (!path.startsWith(`${root}/`)) throw new Error(`artifact escapes release root: ${artifact.path}`);
  const bytes = await readFile(path);
  const actual = createHash("sha256").update(bytes).digest("hex");
  if (actual !== artifact.sha256 || (await stat(path)).size !== artifact.bytes) throw new Error(`artifact integrity failed: ${artifact.path}`);
}
console.log(`verified ${manifest.artifacts.length} artifacts for ${expectedVersion} at ${expectedCommit}`);
