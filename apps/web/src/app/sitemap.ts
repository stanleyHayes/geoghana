import type { MetadataRoute } from "next";
import { isIndexable, siteOrigin } from "@/lib/seo";

const routes = ["", "/products", "/developers", "/coverage", "/support", "/docs", "/changelog", "/status", "/about", "/contact", "/transparency"];

export default function sitemap(): MetadataRoute.Sitemap {
  if (!isIndexable) return [];
  return routes.map((route) => ({
    url: `${siteOrigin}${route}`,
    changeFrequency: route === "/status" ? "hourly" : route === "/changelog" ? "weekly" : "monthly",
    priority: route === "" ? 1 : route === "/docs" || route === "/developers" ? 0.9 : 0.7,
  }));
}
