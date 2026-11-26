import type { Metadata } from "next";
import "./globals.css";
import { Providers } from "@/components/providers";
import UpdateBanner from "@/components/layout/UpdateBanner";
import TrialBanner from "@/components/layout/TrialBanner";

export const metadata: Metadata = {
  title: "Kuenta",
  description: "Kuenta",
  applicationName: "Kuenta",
  icons: {
    icon: [
      { url: "/favicon.ico", sizes: "any" },
      { url: "/favicon.svg", type: "image/svg+xml" },
    ],
    apple: "/kuenta-180.png",
  },
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body suppressHydrationWarning>
        <Providers>
          <UpdateBanner />
          <TrialBanner />
          {children}
        </Providers>
      </body>
    </html>
  );
}
