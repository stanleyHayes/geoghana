import type { Metadata } from "next";
import { JetBrains_Mono, Outfit } from "next/font/google";
import { ThemeProvider, themeInitScript } from "@ghanageo/ui";
import "./globals.css";

// latin-ext is REQUIRED, not optional: the `latin` subset alone drops the
// Latin Extended characters Twi, Ga and Ewe place names need. A place-names
// product for Ghana rendering "Ɔsu" as a tofu box is broken.
// (DESIGN_SYSTEM.md 4.4.1)
const sans = Outfit({
  subsets: ["latin", "latin-ext"],
  variable: "--font-outfit",
  display: "swap",
});
const mono = JetBrains_Mono({
  subsets: ["latin", "latin-ext"],
  variable: "--font-jetbrains",
  display: "swap",
});

export const metadata: Metadata = {
  metadataBase: new URL("https://geo.digitalghana.dev"),
  title: "GhanaGeo — Ghana's open location infrastructure",
  description: "Search Ghana's regions, districts, towns and boundaries through one open, source-aware location layer.",
  openGraph: {
    title: "GhanaGeo — Ghana's open location infrastructure",
    description: "One open, source-aware layer for Ghana's regions, districts, places and boundaries.",
    url: "/",
    siteName: "GhanaGeo",
    locale: "en_GH",
    type: "website",
  },
  twitter: { card: "summary_large_image" },
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning className={`${sans.variable} ${mono.variable}`}>
      <head>
        {/* Blocking and inline, before any painted content. A deferred script
            runs after first paint, which is the bug this avoids. */}
        <script dangerouslySetInnerHTML={{ __html: themeInitScript() }} />
        <script type="application/ld+json" dangerouslySetInnerHTML={{ __html: JSON.stringify({
          "@context": "https://schema.org",
          "@graph": [
            { "@type": "Organization", "@id": "https://geo.digitalghana.dev/#organization", name: "GhanaGeo", url: "https://geo.digitalghana.dev", email: "support@digitalghana.dev" },
            { "@type": "WebSite", "@id": "https://geo.digitalghana.dev/#website", name: "GhanaGeo", url: "https://geo.digitalghana.dev", publisher: { "@id": "https://geo.digitalghana.dev/#organization" } },
            { "@type": "SoftwareApplication", name: "GhanaGeo API", applicationCategory: "DeveloperApplication", operatingSystem: "Web", isAccessibleForFree: true, url: "https://geo.digitalghana.dev/developers" },
          ],
        }) }} />
      </head>
      <body>
        <ThemeProvider>{children}</ThemeProvider>
      </body>
    </html>
  );
}
