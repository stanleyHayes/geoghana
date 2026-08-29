"use client";

import { GhanaGeoProvider } from "@ghanageo/react";
import { resolveApiBase } from "@ghanageo/ui";

const API = resolveApiBase(process.env.NEXT_PUBLIC_GHANAGEO_API_URL);

export function SandboxProviders({ children }: { children: React.ReactNode }) {
  return <GhanaGeoProvider options={{ baseUrl: API }}>{children}</GhanaGeoProvider>;
}
