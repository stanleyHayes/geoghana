import { cpSync, mkdirSync, realpathSync, readdirSync, writeFileSync } from "node:fs";
import { basename, resolve } from "node:path";

const [consumerArgument, releaseArgument, version] = process.argv.slice(2);
if (!consumerArgument || !releaseArgument || !/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-(?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*))*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$/.test(version ?? "")) {
  throw new Error("usage: prepare-sdk-consumer.mjs CONSUMER_DIR RELEASE_DIR VERSION");
}
const consumer = resolve(consumerArgument);
const release = resolve(releaseArgument);
mkdirSync(consumer, { recursive: true });
const consumerReal = realpathSync(consumer);
const repositoryRoot = realpathSync(resolve(import.meta.dirname, ".."));
if (consumerReal === repositoryRoot || !basename(consumerReal).startsWith("ghanageo-sdk-consumer.")) {
  throw new Error(`refusing unsafe consumer directory: ${consumerReal}`);
}
if (readdirSync(consumerReal).length !== 0) {
  throw new Error(`consumer directory must be empty: ${consumerReal}`);
}

const archive = (name) => `file:${resolve(release, `${name}-${version}.tgz`)}`;
const packageJson = {
  name: "ghanageo-release-consumer",
  private: true,
  version: "0.0.0",
  type: "module",
  scripts: {
    runtime: "node runtime.mjs",
    examples: "tsc -p tsconfig.json",
  },
  dependencies: {
    "@ghanageo/core": archive("ghanageo-core"),
    "@ghanageo/client": archive("ghanageo-client"),
    "@ghanageo/react": archive("ghanageo-react"),
    "@ghanageo/node": archive("ghanageo-node"),
    "@ghanageo/data": archive("ghanageo-data"),
    "@ghanageo/proto": archive("ghanageo-proto"),
    ghanageo: archive("ghanageo"),
    "@bufbuild/protobuf": "2.10.2",
    "@tanstack/react-query": "5.102.8",
    react: "19.2.8",
    "react-dom": "19.2.8",
  },
  devDependencies: {
    "@types/react": "19.2.7",
    "@types/react-dom": "19.2.3",
    typescript: "5.9.3",
  },
};
writeFileSync(resolve(consumer, "package.json"), `${JSON.stringify(packageJson, null, 2)}\n`);
cpSync(resolve(import.meta.dirname, "sdk-consumer/runtime.mjs"), resolve(consumer, "runtime.mjs"));
cpSync(resolve(import.meta.dirname, "sdk-consumer/tsconfig.json"), resolve(consumer, "tsconfig.json"));
cpSync(resolve(import.meta.dirname, "../packages/react/examples"), resolve(consumer, "examples"), { recursive: true });
