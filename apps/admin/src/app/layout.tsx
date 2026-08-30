import type { Metadata } from "next";
import { connection } from "next/server";
import { headers } from "next/headers";
import { JetBrains_Mono, Outfit } from "next/font/google";
import { ThemeProvider, themeInitScript } from "@ghanageo/ui";
import { AuthenticatedAdmin } from "@/components/authenticated-admin";
import { SessionProvider } from "@/components/session";
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
  title: "GhanaGeo Admin",
  description: "Curate, review, reconcile, publish and audit Ghana's canonical location data.",
  robots: { index: false, follow: false, nocache: true },
};

// Operator HTML and RSC payloads can contain privileged operational data.
// Make the route tree explicitly non-cacheable in Next's final render policy;
// proxy headers alone can be replaced by the App Router while streaming.
export const revalidate = 0;
export const fetchCache = "force-no-store";

export default async function RootLayout({ children }: { children: React.ReactNode }) {
  await connection();
  const nonce = (await headers()).get("x-nonce") ?? undefined;
  return (
    <html lang="en" suppressHydrationWarning className={`${sans.variable} ${mono.variable}`}>
      <head>
        {/* Blocking and inline, before any painted content. A deferred script
            runs after first paint, which is the bug this avoids. */}
        <script nonce={nonce} dangerouslySetInnerHTML={{ __html: themeInitScript() }} />
      </head>
      <body>
        <ThemeProvider>
          {/* The shell lives in the layout so the rail, navbar and palette
              persist across navigation instead of remounting per page. */}
          {/* Who is signed in and what they may do. The permission list
              gates what the console RENDERS; the API enforces it again. */}
          <SessionProvider>
            <AuthenticatedAdmin>{children}</AuthenticatedAdmin>
          </SessionProvider>
        </ThemeProvider>
      </body>
    </html>
  );
}
