import type { Metadata } from "next";
import "./globals.css";
import { Providers } from "@/components/providers";
import UpdateBanner from "@/components/layout/UpdateBanner";

export const metadata: Metadata = {
  title: "Laguna Escondida",
  description: "Full-stack application",
  icons: {
    icon: [
      { url: "/favicon.ico", sizes: "any" },
      { url: "/favicon.svg", type: "image/svg+xml" },
    ],
    apple: "/logo-navbar.png",
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
          {children}
        </Providers>
      </body>
    </html>
  );
}
