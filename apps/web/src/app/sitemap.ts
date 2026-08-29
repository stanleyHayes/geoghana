import type { MetadataRoute } from "next";

const origin = "https://geo.digitalghana.dev";
const routes = ["", "/products", "/developers", "/coverage", "/support", "/docs", "/changelog", "/status", "/about", "/contact", "/transparency"];

export default function sitemap(): MetadataRoute.Sitemap {
  return routes.map((route) => ({
    url: `${origin}${route}`,
    lastModified: new Date("2026-08-29T00:00:00Z"),
    changeFrequency: route === "/status" ? "hourly" : route === "/changelog" ? "weekly" : "monthly",
    priority: route === "" ? 1 : route === "/docs" || route === "/developers" ? 0.9 : 0.7,
  }));
}
