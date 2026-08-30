#!/usr/bin/env node

import { createHash } from "node:crypto";
import { readFile, readdir, stat, writeFile } from "node:fs/promises";
import { basename, relative, resolve } from "node:path";
import { execFileSync } from "node:child_process";

const [releaseRootArg, version] = process.argv.slice(2);
if (!releaseRootArg || !/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$/.test(version ?? "")) {
  throw new Error("usage: generate-sdk-release-manifest.mjs <release-root> <semver>");
}

const releaseRoot = resolve(releaseRootArg);
const repoRoot = resolve(import.meta.dirname, "..");

async function filesUnder(directory) {
  const entries = await readdir(directory, { withFileTypes: true });
  const nested = await Promise.all(entries.map(async (entry) => {
    const path = resolve(directory, entry.name);
    return entry.isDirectory() ? filesUnder(path) : [path];
  }));
  return nested.flat();
}

async function digest(path) {
  return createHash("sha256").update(await readFile(path)).digest("hex");
}

async function digestSet(paths) {
  const hash = createHash("sha256");
  for (const path of [...paths].sort()) {
    hash.update(relative(repoRoot, path));
    hash.update("\0");
    hash.update(await readFile(path));
    hash.update("\0");
  }
  return hash.digest("hex");
}

const artifactFiles = (await filesUnder(releaseRoot))
  .filter((path) => basename(path) !== "compatibility-manifest.json")
  .sort();
const expectedCounts = { npm: 7, python: 2, go: 1, dart: 2, java: 1, dotnet: 2, php: 1 };
for (const [family, expected] of Object.entries(expectedCounts)) {
  const actual = artifactFiles.filter((path) => relative(releaseRoot, path).startsWith(`${family}/`)).length;
  if (actual !== expected) throw new Error(`${family}: expected ${expected} artifacts, found ${actual}`);
}
const contractFiles = (await Promise.all([
  filesUnder(resolve(repoRoot, "contracts")),
  filesUnder(resolve(repoRoot, "proto")),
])).flat().filter((path) => !path.includes("node_modules")).sort();
const conformanceFiles = await filesUnder(resolve(repoRoot, "contracts/conformance"));
const coreSource = await readFile(resolve(repoRoot, "packages/core/src/index.ts"), "utf8");
const readText = (path) => readFile(resolve(repoRoot, path), "utf8");
const capture = (text, pattern, label) => {
  const value = text.match(pattern)?.[1];
  if (!value) throw new Error(`cannot read SDK version from ${label}`);
  return value;
};
const npmPackages = ["core", "client", "react", "node", "data", "proto", "cli"];
const sourceNpmVersions = Object.fromEntries(await Promise.all(npmPackages.map(async (name) => {
  const manifest = JSON.parse(await readText(`packages/${name}/package.json`));
  return [manifest.name, manifest.version];
})));
const npmArchives = artifactFiles.filter((path) => relative(releaseRoot, path).startsWith("npm/") && path.endsWith(".tgz"));
let npmVersions = sourceNpmVersions;
try {
  if (npmArchives.length !== 7) throw new Error("staged npm tarballs are incomplete");
  npmVersions = Object.fromEntries(npmArchives.map((path) => {
    const packageJson = execFileSync("tar", ["-xOf", path, "package/package.json"], { encoding: "utf8" });
    const manifest = JSON.parse(packageJson);
    return [manifest.name, manifest.version];
  }));
} catch {
  if (process.env.GHANAGEO_SYNTHETIC_RELEASE !== "true") throw new Error("cannot read npm versions from staged tarballs");
}
const pythonProject = await readText("sdks/python/pyproject.toml");
const pythonVersion = await readText("sdks/python/src/ghanageo/_version.py");
const goVersion = await readText("sdks/go/version.go");
const dartProject = await readText("sdks/dart/pubspec.yaml");
const flutterProject = await readText("sdks/dart/packages/ghanageo_flutter/pubspec.yaml");
const javaProject = await readText("sdks/java/pom.xml");
const javaVersion = await readText("sdks/java/client/src/main/java/dev/ghanageo/Version.java");
const dotnetProject = await readText("sdks/dotnet/src/GhanaGeo/GhanaGeo.csproj");
const dotnetVersion = await readText("sdks/dotnet/src/GhanaGeo/GhanaGeoClient.cs");
const phpVersion = await readText("sdks/php/src/Version.php");
const javaArchive = artifactFiles.find((path) => relative(releaseRoot, path).startsWith("java/") && path.endsWith("-maven-bundle.tar.gz"));
let stagedJavaProject = javaProject;
let stagedJavaVersion = javaVersion;
try {
  if (!javaArchive) throw new Error("Java bundle is missing");
  stagedJavaProject = execFileSync("tar", ["-xOf", javaArchive, "./pom.xml"], { encoding: "utf8" });
  stagedJavaVersion = execFileSync("tar", ["-xOf", javaArchive, "./client/src/main/java/dev/ghanageo/Version.java"], { encoding: "utf8" });
} catch {
  if (process.env.GHANAGEO_SYNTHETIC_RELEASE !== "true") throw new Error("cannot read Java metadata from staged Maven bundle");
  stagedJavaProject = javaProject.replaceAll("2.0.0-SNAPSHOT", version);
  stagedJavaVersion = javaVersion.replaceAll("2.0.0-SNAPSHOT", version);
}
const tsApi = capture(coreSource, /API_VERSION\s*=\s*["']([^"']+)/, "TypeScript API");
const tsDataset = capture(coreSource, /TESTED_DATASET_VERSION\s*=\s*["']([^"']+)/, "TypeScript dataset");
const sdkMetadata = {
  typescript: { packages: npmVersions, install: Object.entries(npmVersions).map(([name, value]) => `npm install ${name}@${value}`), apiTarget: tsApi, testedDataset: tsDataset },
  python: {
    codeVersion: capture(pythonVersion, /__version__\s*=\s*"([^"]+)"/, "Python code"),
    apiTarget: capture(pythonVersion, /api_version\s*=\s*"([^"]+)"/, "Python API"),
    testedDataset: capture(pythonVersion, /tested_dataset_version\s*=\s*"([^"]+)"/, "Python dataset"),
  },
    go: {
      sourceRepository: "github.com/ghanageo/ghanageo-go",
    codeVersion: capture(goVersion, /SDKVersion\s*=\s*"([^"]+)"/, "Go code"),
    apiTarget: capture(goVersion, /APIVersion\s*=\s*"([^"]+)"/, "Go API"),
    testedDataset: capture(goVersion, /TestedDatasetVersion\s*=\s*"([^"]+)"/, "Go dataset"),
  },
  dart: {
    codeVersion: capture(dartProject, /^version:\s*([^\s]+)/m, "Dart code"),
    apiTarget: capture(await readText("sdks/dart/lib/src/client.dart"), /apiVersion\s*=\s*'([^']+)'/, "Dart API"),
    testedDataset: capture(await readText("sdks/dart/lib/src/client.dart"), /testedDatasetVersion\s*=\s*'([^']+)'/, "Dart dataset"),
  },
  flutter: {
    codeVersion: capture(flutterProject, /^version:\s*([^\s]+)/m, "Flutter code"),
    apiTarget: capture(await readText("sdks/dart/lib/src/client.dart"), /apiVersion\s*=\s*'([^']+)'/, "Flutter API via Dart dependency"),
    testedDataset: capture(await readText("sdks/dart/lib/src/offline_regions.dart"), /offlineRegionsDatasetVersion\s*=\s*'([^']+)'/, "Flutter dataset"),
  },
  java: {
    codeVersion: capture(stagedJavaVersion, /SDK\s*=\s*"([^"]+)"/, "staged Java code"),
    apiTarget: capture(stagedJavaVersion, /API\s*=\s*"([^"]+)"/, "staged Java API"),
    testedDataset: capture(stagedJavaVersion, /TESTED_DATASET\s*=\s*"([^"]+)"/, "staged Java dataset"),
  },
  dotnet: {
    codeVersion: capture(dotnetVersion, /SdkVersion\s*=\s*"([^"]+)"/, ".NET code"),
    apiTarget: capture(dotnetVersion, /CurrentApiVersion\s*=\s*"([^"]+)"/, ".NET API"),
    testedDataset: capture(dotnetVersion, /CurrentTestedDatasetVersion\s*=\s*"([^"]+)"/, ".NET dataset"),
  },
    php: {
      sourceRepository: "github.com/ghanageo/ghanageo-php",
    codeVersion: capture(phpVersion, /SDK\s*=\s*'([^']+)'/, "PHP code"),
    apiTarget: capture(phpVersion, /API\s*=\s*'([^']+)'/, "PHP API"),
    testedDataset: capture(phpVersion, /TESTED_DATASET\s*=\s*'([^']+)'/, "PHP dataset"),
  },
};
if (capture(pythonProject, /^version\s*=\s*"([^"]+)"/m, "Python package") !== sdkMetadata.python.codeVersion) throw new Error("Python package/runtime versions differ");
if (capture(stagedJavaProject, /<artifactId>ghanageo-java-parent<\/artifactId><version>([^<]+)<\/version>/, "staged Java package") !== sdkMetadata.java.codeVersion) throw new Error("staged Java package/runtime versions differ");
if (/SNAPSHOT/i.test(sdkMetadata.java.codeVersion)) throw new Error("staged Java version must never be SNAPSHOT");
if (capture(dotnetProject, /<Version>([^<]+)<\/Version>/, ".NET package") !== sdkMetadata.dotnet.codeVersion) throw new Error(".NET package/runtime versions differ");
const flutterDependency = capture(flutterProject, /ghanageo:\s*\^([^\s]+)/, "Flutter Dart dependency");
if (flutterDependency !== sdkMetadata.dart.codeVersion) throw new Error("Flutter install dependency does not match Dart artifact version");
sdkMetadata.python.install = `python -m pip install ghanageo==${sdkMetadata.python.codeVersion}`;
sdkMetadata.go.install = `go get github.com/ghanageo/ghanageo-go@v${sdkMetadata.go.codeVersion}`;
sdkMetadata.dart.install = `dart pub add ghanageo:${sdkMetadata.dart.codeVersion}`;
sdkMetadata.flutter.install = `flutter pub add ghanageo_flutter:${sdkMetadata.flutter.codeVersion}`;
sdkMetadata.java.install = `implementation("dev.ghanageo:ghanageo-java:${sdkMetadata.java.codeVersion}")`;
sdkMetadata.dotnet.install = `dotnet add package GhanaGeo --version ${sdkMetadata.dotnet.codeVersion}`;
sdkMetadata.php.install = `composer require ghanageo/ghanageo-php:${sdkMetadata.php.codeVersion}`;
const prerelease = (value) => /(?:-|\d(?:a|b|rc)\d|SNAPSHOT)/i.test(value);
const codeVersions = [...Object.values(npmVersions), ...Object.values(sdkMetadata).filter((item) => "codeVersion" in item).map((item) => item.codeVersion)];
if (!version.includes("-") && codeVersions.some(prerelease)) throw new Error("stable coordinated releases cannot contain prerelease or SNAPSHOT SDK versions");
const git = (args, fallback) => {
  try { return execFileSync("git", args, { cwd: repoRoot, encoding: "utf8" }).trim(); }
  catch { return fallback; }
};

const manifest = {
  schemaVersion: 1,
  release: {
    version,
    tag: `v${version}`,
    commit: git(["rev-parse", "HEAD"], process.env.GITHUB_SHA ?? "unknown"),
    sourceTree: git(["status", "--porcelain"], "unknown") === "" ? "clean" : "dirty",
    apiTarget: "v1",
    testedDataset: tsDataset,
  },
  contracts: {
    aggregateSha256: await digestSet(contractFiles),
    conformanceSha256: await digestSet(conformanceFiles),
    files: contractFiles.map((path) => relative(repoRoot, path)),
  },
  sdks: sdkMetadata,
  artifacts: await Promise.all(artifactFiles.map(async (path) => ({
    path: relative(releaseRoot, path),
    bytes: (await stat(path)).size,
    sha256: await digest(path),
    provenance: process.env.GITHUB_ACTIONS === "true" ? "github-actions-oidc-eligible" : "local-rehearsal",
    signed: false,
    signingStatus: "not-signed-rehearsal",
  }))),
  publication: {
    attempted: false,
    registries: ["npm", "PyPI", "Go module proxy", "pub.dev", "Maven Central", "NuGet", "Packagist"],
  },
};

await writeFile(resolve(releaseRoot, "compatibility-manifest.json"), `${JSON.stringify(manifest, null, 2)}\n`);
console.log(`wrote ${releaseRoot}/compatibility-manifest.json (${manifest.artifacts.length} artifacts)`);
