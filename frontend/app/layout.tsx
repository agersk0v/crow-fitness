import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Crow Fitness",
  description: "Your personal workout tracker. Project foundation preview.",
};

export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
