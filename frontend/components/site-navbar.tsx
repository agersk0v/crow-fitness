"use client";

import { AuthControls, useAuth } from "@/components/auth-boundary";
import Link from "next/link";
import { usePathname, useSearchParams } from "next/navigation";
import { ThemeToggle } from "@/components/theme-toggle";

const destinations = [
  { label: "Home", href: "/", section: "home" },
  { label: "Exercises", href: "/exercises", section: "exercises" },
  { label: "Templates", href: "/templates", section: "templates" },
  { label: "Workout history", href: "/history", section: "history" },
];

export function Navbar({
  selected = "",
  historySearch = "",
  pointerControls = false,
  activeSessionId = null,
}: {
  selected?: string;
  historySearch?: string;
  pointerControls?: boolean;
  activeSessionId?: string | null;
}) {
  return (
    <header
      className={`mx-auto max-w-5xl px-6 pt-8 sm:px-12 ${pointerControls ? "[&_button:enabled]:cursor-pointer [&_a[href]]:cursor-pointer" : ""}`}
    >
      <div className="flex flex-wrap items-center justify-between gap-4 border-b border-border pb-6">
        <nav aria-label="Main navigation" className="flex flex-wrap gap-2">
          {[
            ...destinations,
            ...(activeSessionId
              ? [
                  {
                    label: "Active session",
                    href: `/sessions/${activeSessionId}`,
                    section: "active",
                  },
                ]
              : []),
          ].map(({ label, href, section }) => (
            <Link
              key={section}
              href={
                section === "history" && historySearch
                  ? `/history?${historySearch}`
                  : href
              }
              aria-current={selected === section ? "page" : undefined}
              className={`inline-flex min-h-11 items-center rounded-lg px-3 text-sm text-highlight hover:bg-primary/5 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring ${selected === section ? "bg-primary/10 font-semibold underline underline-offset-4" : ""}`}
            >
              {label}
            </Link>
          ))}
        </nav>
        <div className="flex items-center gap-2">
          <AuthControls />
          <ThemeToggle />
        </div>
      </div>
    </header>
  );
}

export function SiteNavbar() {
  const auth = useAuth();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const selected =
    pathname === "/"
      ? "home"
      : pathname.startsWith("/exercises")
        ? "exercises"
        : pathname.startsWith("/sessions")
          ? "active"
          : pathname.startsWith("/templates")
            ? "templates"
            : pathname.startsWith("/history")
              ? "history"
              : "";
  return (
    <Navbar
      activeSessionId={
        auth.status === "signed-in" ? auth.activeSessionId : null
      }
      selected={selected}
      pointerControls={
        pathname.startsWith("/templates") ||
        pathname.startsWith("/history") ||
        pathname.startsWith("/sessions")
      }
      historySearch={pathname === "/history" ? searchParams.toString() : ""}
    />
  );
}
