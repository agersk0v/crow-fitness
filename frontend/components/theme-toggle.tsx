"use client";

import { useEffect, useState } from "react";
import { useTheme } from "next-themes";
import { Button } from "@/components/ui/button";

export function ThemeToggle() {
  const { resolvedTheme, setTheme } = useTheme();
  const [mounted, setMounted] = useState(false);

  useEffect(() => setMounted(true), []);

  const nextTheme = mounted && resolvedTheme === "light" ? "dark" : "light";

  return (
    <Button
      type="button"
      variant="outline"
      disabled={!mounted}
      onClick={() => setTheme(nextTheme)}
      aria-label={`Switch to ${nextTheme} mode`}
      className="min-h-11 min-w-28"
    >
      {nextTheme === "light" ? "Light mode" : "Dark mode"}
    </Button>
  );
}
