#!/usr/bin/env node
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const root = resolve(import.meta.dirname, "..");
const read = (path) => readFileSync(resolve(root, path), "utf8");
const manifest = JSON.parse(read("package.json"));
assert.equal(manifest.name, "ghanageo");
assert.equal(manifest.private, true, "root workspace must remain private");

const legacy = [".github/workflows/sdk-release.yml", ".github/workflows/cli-release.yml"].map((path) => [path, read(path)]);
for (const [path, workflow] of legacy) {
  assert.doesNotMatch(workflow, /^\s+tags:/m, `${path} must be manual dry-run only`);
  assert.doesNotMatch(workflow, /npm publish|\bgh release create\b/, `${path} must not mutate registries`);
  assert.doesNotMatch(workflow, /npm(?:\s+--prefix\s+\S+)?\s+version\b/, `${path} must not stamp source manifests`);
}

const publisher = read(".github/workflows/v2-sdk-publish.yml");
const npmPublisher = read("scripts/publish-npm-sdk-bundle.sh");
assert.match(publisher, /publish-npm-sdk-bundle\.sh bundle/, "protected V2 publisher must own all npm publication");
assert.match(npmPublisher, /for package in core client react node data proto;/, "scoped npm dependency order is incomplete");
assert.match(npmPublisher, /verify_or_publish ghanageo [^\n]*ghanageo-\$version\.tgz[^\n]*npm-umbrella/, "V2 publisher must publish or integrity-verify the CLI umbrella");
assert.match(npmPublisher, /dist\.shasum/, "npm resume must verify registry tarball SHA-1");
assert.match(npmPublisher, /dist\.integrity/, "npm resume must verify registry tarball integrity");
const allWorkflowText = legacy.map(([, value]) => value).join("\n") + publisher;
assert.equal((allWorkflowText.match(/publish-npm-sdk-bundle\.sh bundle/g) ?? []).length, 1, "exactly one workflow may own the seven npm publications");
console.log("verified V2 sole seven-package npm ownership and dry-run-only legacy workflows");
