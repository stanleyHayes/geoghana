import type { NextConfig } from "next";

/**
 * The API is proxied under /api rather than called cross-origin.
 *
 * The session cookie is HttpOnly and SameSite=Lax, so a fetch from
 * localhost:3103 to localhost:8180 would simply not carry it — sign-in would
 * appear to work and every authenticated request would come back 401. Proxying
 * makes the console same-origin with its own API, which is also what a
 * production deployment behind one hostname looks like, so local and
 * production behave the same way instead of diverging.
 */
const API_ORIGIN = process.env.GHANAGEO_API_ORIGIN ?? "http://localhost:8180";

const config: NextConfig = {
  transpilePackages: ["@ghanageo/ui"],
  reactStrictMode: true,
  async rewrites() {
    return [
      { source: "/api/v1/:path*", destination: `${API_ORIGIN}/v1/:path*` },
      { source: "/api/graphql", destination: `${API_ORIGIN}/graphql` },
    ];
  },
};
export default config;
