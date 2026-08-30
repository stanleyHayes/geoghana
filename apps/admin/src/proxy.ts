import { NextResponse, type NextRequest } from "next/server";
import { safeReturnTo } from "@/lib/safe-return-to";
import { adminApiOrigin } from "@/lib/runtime-config";

const API_ORIGIN = adminApiOrigin();
const ADMIN_ROLES = new Set([
  "SUPER_ADMIN", "DATA_ADMIN", "DATA_REVIEWER", "DATA_CONTRIBUTOR",
  "DEVELOPER_SUPPORT", "SECURITY_AUDITOR",
]);

function loginRedirect(request: NextRequest) {
  const url = request.nextUrl.clone();
  url.pathname = "/login";
  url.search = "";
  url.searchParams.set("returnTo", safeReturnTo(`${request.nextUrl.pathname}${request.nextUrl.search}`, request.nextUrl.origin));
  return NextResponse.redirect(url);
}

function securityContext(request: NextRequest) {
  const nonce = Buffer.from(crypto.randomUUID()).toString("base64");
  const apiOrigin = new URL(API_ORIGIN).origin;
  const csp = [
    "default-src 'self'",
    `script-src 'self' 'nonce-${nonce}' 'strict-dynamic'${process.env.NODE_ENV === "development" ? " 'unsafe-eval'" : ""}`,
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' blob: data:",
    "font-src 'self' data:",
    `connect-src 'self' ${apiOrigin}`,
    "object-src 'none'",
    "base-uri 'self'",
    "form-action 'self'",
    "frame-ancestors 'none'",
    ...(process.env.NODE_ENV === "production" ? ["upgrade-insecure-requests"] : []),
  ].join("; ");
  const requestHeaders = new Headers(request.headers);
  requestHeaders.set("x-nonce", nonce);
  requestHeaders.set("Content-Security-Policy", csp);
  return { requestHeaders, csp };
}

function securedNext(requestHeaders: Headers, csp: string, noStore = false) {
  const response = NextResponse.next({ request: { headers: requestHeaders } });
  response.headers.set("Content-Security-Policy", csp);
  if (noStore) response.headers.set("Cache-Control", "private, no-store, max-age=0");
  return response;
}

function secureResponse(response: NextResponse, csp: string, noStore = true) {
  response.headers.set("Content-Security-Policy", csp);
  if (noStore) response.headers.set("Cache-Control", "private, no-store, max-age=0");
  return response;
}

export async function proxy(request: NextRequest) {
  const { requestHeaders, csp } = securityContext(request);
  if (request.nextUrl.pathname === "/login" || request.nextUrl.pathname === "/access-denied") {
    return securedNext(requestHeaders, csp, true);
  }
  const sessionCookie = request.cookies.get("gg_session")?.value;
  if (!sessionCookie) return secureResponse(loginRedirect(request), csp);

  try {
    const validation = await fetch(`${API_ORIGIN}/v1/auth/session`, {
      headers: {
        cookie: `gg_session=${sessionCookie}`,
        "user-agent": request.headers.get("user-agent") ?? "GhanaGeo Admin",
        "x-forwarded-for": request.headers.get("x-forwarded-for") ?? "",
        "x-forwarded-proto": request.nextUrl.protocol.replace(":", ""),
      },
      cache: "no-store",
    });
    if (!validation.ok) {
      const response = loginRedirect(request);
      response.cookies.delete("gg_session");
      return secureResponse(response, csp);
    }

    const body = await validation.json().catch(() => null) as { data?: { role?: string } } | null;
    if (!body?.data?.role || !ADMIN_ROLES.has(body.data.role)) {
      const url = request.nextUrl.clone();
      url.pathname = "/access-denied";
      url.search = "";
      const response = NextResponse.redirect(url);
      const rotatedCookie = validation.headers.get("set-cookie");
      if (rotatedCookie) response.headers.append("set-cookie", rotatedCookie);
      return secureResponse(response, csp);
    }

    const response = securedNext(requestHeaders, csp);
    const rotatedCookie = validation.headers.get("set-cookie");
    if (rotatedCookie) response.headers.append("set-cookie", rotatedCookie);
    response.headers.set("Cache-Control", "private, no-store");
    return response;
  } catch {
    return secureResponse(new NextResponse("The admin authentication service is unavailable.", {
      status: 503,
      headers: { "Cache-Control": "no-store", "Retry-After": "30" },
    }), csp);
  }
}

export const config = {
  matcher: ["/((?!api|_next/static|_next/image|icon.svg|favicon.ico).*)"],
};
