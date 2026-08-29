import type { Metadata } from "next";
import { Fraunces, JetBrains_Mono, Outfit } from "next/font/google";
import { ThemeProvider, themeInitScript } from "@ghanageo/ui";
import "./globals.css";

// latin-ext is REQUIRED, not optional: the `latin` subset alone drops the
// Latin Extended characters Twi, Ga and Ewe place names need. A place-names
// product for Ghana rendering "Ɔsu" as a tofu box is broken.
// (DESIGN_SYSTEM.md 4.4.1)
// Fraunces is a variable serif with an optical-size axis: at display sizes it
// tightens and gains contrast, at small sizes it opens up. Requesting `opsz`
// lets the browser do that automatically instead of us shipping two families.
const display = Fraunces({
  subsets: ["latin", "latin-ext"],
  variable: "--font-fraunces",
  display: "swap",
  // A variable font may declare axes OR explicit weights, not both. Keeping
  // the axes means the whole weight range stays available and the optical-size
  // axis works, which is the reason for choosing Fraunces.
  axes: ["opsz", "SOFT", "WONK"],
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
  title: "GhanaGeo — Ghana's open location infrastructure",
  description: "Search Ghana's regions, districts, towns and boundaries through one open, source-aware location layer.",
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
