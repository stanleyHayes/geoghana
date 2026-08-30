import type { MetadataRoute } from "next";

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "GhanaGeo — Ghana's open location infrastructure",
    short_name: "GhanaGeo",
    description: "Open, source-aware location data and developer tools for Ghana.",
    start_url: "/",
    display: "standalone",
    background_color: "#071f19",
    theme_color: "#45c4a1",
    icons: [{ src: "/icon.svg", sizes: "any", type: "image/svg+xml", purpose: "any" }],
  };
}
