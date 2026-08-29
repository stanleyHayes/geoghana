import type { Metadata } from "next";
import { Bricolage_Grotesque, JetBrains_Mono, Outfit } from "next/font/google";
import { ThemeProvider, themeInitScript } from "@ghanageo/ui";
import "./globals.css";

// latin-ext is REQUIRED, not optional: the `latin` subset alone drops the
// Latin Extended characters Twi, Ga and Ewe place names need. A place-names
// product for Ghana rendering "Ɔsu" as a tofu box is broken.
// (DESIGN_SYSTEM.md 4.4.1)
const display = Bricolage_Grotesque({
  subsets: ["latin", "latin-ext"],
  variable: "--font-bricolage",
  display: "swap",
});
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
  title: "GhanaGeo — portal",
  description: "Curate, review, reconcile, publish and audit Ghana's canonical location data.",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning className={`${display.variable} ${sans.variable} ${mono.variable}`}>
      <head>
        {/* Blocking and inline, before any painted content. A deferred script
            runs after first paint, which is the bug this avoids. */}
        <script dangerouslySetInnerHTML={{ __html: themeInitScript() }} />
      </head>
      <body>
        <ThemeProvider>{children}</ThemeProvider>
      </body>
    </html>
  );
}
