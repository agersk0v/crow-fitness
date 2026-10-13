import { Suspense } from "react";
import { Navbar, SiteNavbar } from "@/components/site-navbar";
import type { Metadata } from "next";
import "./globals.css";
import { Toaster } from "@/components/ui/sonner";
import { ThemeProvider } from "@/components/theme-provider";

export const metadata: Metadata = {
  title: "Crow Fitness",
  description: "Your personal workout tracker. Project foundation preview.",
};

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en" className="dark" suppressHydrationWarning>
      <body>
        <ThemeProvider>
          <Suspense fallback={<Navbar />}>
            <SiteNavbar />
          </Suspense>
          {children}
          <Toaster
            position="top-center"
            closeButton
            duration={6000}
            toastOptions={{
              classNames: { title: "break-words whitespace-normal" },
            }}
          />
        </ThemeProvider>
      </body>
    </html>
  );
}
