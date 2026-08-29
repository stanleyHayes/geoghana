import { GhanaGeoClient, type GhanaGeoClientOptions } from "@ghanageo/client";

export * from "@ghanageo/client";

export type GhanaGeoEnvironment = {
  GHANAGEO_API_URL?: string;
  GHANAGEO_API_KEY?: string;
};

export function assertServerOnly(): void {
  if ("window" in globalThis) {
    throw new Error("@ghanageo/node cannot run in a browser bundle. Use @ghanageo/client or @ghanageo/react instead.");
  }
}

export function createServerClient(options: GhanaGeoClientOptions = {}): GhanaGeoClient {
  assertServerOnly();
  return new GhanaGeoClient(options);
}

export function createServerClientFromEnv(environment?: GhanaGeoEnvironment): GhanaGeoClient {
  assertServerOnly();
  const resolved = environment ?? {
    ...(process.env.GHANAGEO_API_URL ? { GHANAGEO_API_URL: process.env.GHANAGEO_API_URL } : {}),
    ...(process.env.GHANAGEO_API_KEY ? { GHANAGEO_API_KEY: process.env.GHANAGEO_API_KEY } : {}),
  };
  return new GhanaGeoClient({
    ...(resolved.GHANAGEO_API_URL ? { baseUrl: resolved.GHANAGEO_API_URL } : {}),
    ...(resolved.GHANAGEO_API_KEY ? { apiKey: resolved.GHANAGEO_API_KEY } : {}),
  });
}
