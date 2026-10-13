"use client";

import Link from "next/link";
import { usePathname, useSearchParams } from "next/navigation";
import { ThemeToggle } from "@/components/theme-toggle";

const destinations = [
  { label: "Home", href: "/", section: "home" },
  { label: "Exercises", href: "/exercises", section: "exercises" },
  { label: "Templates", href: "/templates", section: "templates" },
  { label: "Workout history", href: "/history", section: "history" },
  { label: "Sets", href: "/history?tab=sets", section: "sets" },
];

export function Navbar({ selected = "" }: { selected?: string }) {
  return (
    <header className="mx-auto max-w-5xl px-6 pt-8 sm:px-12">
      <div className="flex flex-wrap items-center justify-between gap-4 border-b border-border pb-6">
        <nav aria-label="Main navigation" className="flex flex-wrap gap-2">
          {destinations.map(({ label, href, section }) => (
            <Link
              key={section}
              href={href}
              aria-current={selected === section ? "page" : undefined}
              className={`inline-flex min-h-11 items-center rounded-lg px-3 text-sm text-highlight hover:bg-primary/5 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring ${selected === section ? "bg-primary/10 font-semibold underline underline-offset-4" : ""}`}
            >
              {label}
            </Link>
          ))}
        </nav>
        <ThemeToggle />
      </div>
    </header>
  );
}

export function SiteNavbar() {
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const selected =
    pathname === "/"
      ? "home"
      : pathname.startsWith("/exercises")
        ? "exercises"
        : pathname.startsWith("/templates") || pathname.startsWith("/sessions")
          ? "templates"
          : pathname === "/history" && searchParams.get("tab") === "sets"
            ? "sets"
            : pathname.startsWith("/history")
              ? "history"
              : "";
  return <Navbar selected={selected} />;
}
