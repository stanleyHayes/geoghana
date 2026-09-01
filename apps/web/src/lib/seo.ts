import type { Metadata } from "next";

const FALLBACK_ORIGIN = "https://geo.digitalghana.dev";

function httpUrl(value: string | undefined, fallback: string): URL {
  try {
    const url = new URL(value ?? fallback);
    if (url.protocol !== "http:" && url.protocol !== "https:") throw new Error("unsupported protocol");
    return url;
  } catch {
    return new URL(fallback);
  }
}

export const siteOrigin = httpUrl(process.env.NEXT_PUBLIC_GHANAGEO_WEB_URL, FALLBACK_ORIGIN).origin;
export const sandboxOrigin = httpUrl(
  process.env.NEXT_PUBLIC_GHANAGEO_SANDBOX_URL,
  process.env.NODE_ENV === "development" ? "http://localhost:3101" : "https://sandbox-geo.digitalghana.dev",
).origin;
export const portalOrigin = httpUrl(
  process.env.NEXT_PUBLIC_GHANAGEO_PORTAL_URL,
  process.env.NODE_ENV === "development" ? "http://localhost:3102" : "https://console-geo.digitalghana.dev",
).origin;
export const isIndexable = process.env.NEXT_PUBLIC_GHANAGEO_INDEXABLE === "true";

type PageMetadata = {
  title: string;
  description: string;
  path: `/${string}` | "/";
};

export function pageMetadata({ title, description, path }: PageMetadata): Metadata {
  return {
    title,
    description,
    alternates: { canonical: path },
    openGraph: {
      title,
      description,
      url: path,
      siteName: "GhanaGeo",
      locale: "en_GH",
      type: "website",
      images: [{ url: "/opengraph-image", width: 1200, height: 630, alt: "GhanaGeo — Ghana's open location infrastructure" }],
    },
    twitter: {
      card: "summary_large_image",
      title,
      description,
      images: ["/opengraph-image"],
    },
  };
}
