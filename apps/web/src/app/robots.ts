import type { MetadataRoute } from "next";
import { isIndexable, siteOrigin } from "@/lib/seo";

export default function robots(): MetadataRoute.Robots {
  return {
    rules: isIndexable
      ? { userAgent: "*", allow: "/" }
      : { userAgent: "*", disallow: "/" },
    sitemap: isIndexable ? `${siteOrigin}/sitemap.xml` : undefined,
    host: isIndexable ? siteOrigin : undefined,
  };
}
