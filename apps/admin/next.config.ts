import type { NextConfig } from "next";
import { adminApiOrigin } from "./src/lib/runtime-config";

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
const API_ORIGIN = adminApiOrigin();

const config: NextConfig = {
  transpilePackages: ["@ghanageo/ui"],
  reactStrictMode: true,
  poweredByHeader: false,
  async headers() {
    return [{
      // Document and auth routes only. Hashed framework assets must retain
      // Next's public immutable cache policy instead of inheriting operator
      // HTML's private/no-store policy.
      source: "/((?!_next/static|_next/image|icon.svg|favicon.ico).*)",
      headers: [
        { key: "X-Content-Type-Options", value: "nosniff" },
        { key: "X-Frame-Options", value: "DENY" },
        { key: "Referrer-Policy", value: "same-origin" },
        { key: "Permissions-Policy", value: "camera=(), geolocation=(), microphone=()" },
        { key: "Cross-Origin-Opener-Policy", value: "same-origin" },
        ...(process.env.NODE_ENV === "production"
          ? [{ key: "Strict-Transport-Security", value: "max-age=63072000; includeSubDomains; preload" }]
          : []),
        { key: "X-Robots-Tag", value: "noindex, nofollow, noarchive" },
        { key: "Cache-Control", value: "private, no-store, max-age=0" },
        // Next 16 normalizes streamed dynamic HTML to `private, no-store` and
        // may remove the redundant max-age directive. These preserved headers
        // keep CDNs and legacy HTTP/1.0 clients on the same fail-closed policy.
        { key: "Surrogate-Control", value: "no-store" },
        { key: "Pragma", value: "no-cache" },
        { key: "Expires", value: "0" },
      ],
    }];
  },
  async rewrites() {
    return [
      { source: "/api/v1/:path*", destination: `${API_ORIGIN}/v1/:path*` },
      { source: "/api/graphql", destination: `${API_ORIGIN}/graphql` },
    ];
  },
};
export default config;
