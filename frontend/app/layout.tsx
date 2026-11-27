import type { Metadata } from "next";
import "@fontsource-variable/manrope/wght.css";
import "./globals.css";
import { Providers } from "@/components/providers";
import UpdateBanner from "@/components/layout/UpdateBanner";
import SubscriptionNotice from "@/components/billing/SubscriptionNotice";

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
    <html lang="es-CO">
      <body suppressHydrationWarning>
        <Providers>
          <UpdateBanner />
          <SubscriptionNotice />
          {children}
        </Providers>
      </body>
    </html>
  );
}
