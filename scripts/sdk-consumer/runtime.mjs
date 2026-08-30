import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { execFileSync } from "node:child_process";
import { resolve } from "node:path";
import { API_VERSION, TESTED_DATASET_VERSION } from "@ghanageo/core";
import { GhanaGeoClient } from "@ghanageo/client";
import * as reactSdk from "@ghanageo/react";
import { createServerClient } from "@ghanageo/node";
import { datasetVersion, getRegions } from "@ghanageo/data";
import * as proto from "@ghanageo/proto";
import * as umbrella from "ghanageo";

assert.equal(API_VERSION, "v1");
assert.equal(TESTED_DATASET_VERSION, datasetVersion);
assert.equal(getRegions().length, 16);
assert.equal(typeof reactSdk.GhanaGeoProvider, "function");
assert.equal(typeof createServerClient({ fetcher: fetch }).regions, "function");
assert.equal(proto.PROTO_API_VERSION, "ghanageo.v1");
assert.ok(Object.keys(proto).length > 1);
assert.equal(umbrella.datasetVersion, datasetVersion);
assert.equal(umbrella.getRegions().length, 16);

let observedUrl = "";
const client = new GhanaGeoClient({
  baseUrl: "https://consumer.invalid/v1",
  retry: false,
  fetcher: async (resource, init) => {
    observedUrl = String(resource);
    assert.equal(new Headers(init?.headers).has("authorization"), false);
    return new Response(JSON.stringify({ data: [], datasetVersion }), { status: 200, headers: { "content-type": "application/json" } });
  },
});
assert.deepEqual(await client.regions({ limit: 1 }), { data: [], datasetVersion });
assert.match(observedUrl, /\/v1\/regions\?limit=1$/);

const require = createRequire(import.meta.url);
const commonJsUmbrella = require("ghanageo");
assert.equal(commonJsUmbrella.datasetVersion, datasetVersion);
const launcher = resolve(import.meta.dirname, "node_modules", ".bin", process.platform === "win32" ? "ghanageo.cmd" : "ghanageo");
const cliVersion = execFileSync(launcher, ["version"], { encoding: "utf8" });
assert.match(cliVersion, /ghanageo (?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)(?:-(?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9]\d*|\d*[A-Za-z-][0-9A-Za-z-]*))*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)? \(targets GhanaGeo API v1\)/);
console.log("fresh consumer runtime imports passed for seven SDK artifacts");
