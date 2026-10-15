"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import type { Session } from "@/lib/sessions";

export function StartSession({
  templateID,
  compact = false,
  beforeStart,
  disabled = false,
  onBusyChange,
}: {
  templateID: string;
  compact?: boolean;
  beforeStart?: () => boolean;
  disabled?: boolean;
  onBusyChange?: (busy: boolean) => void;
}) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [active, setActive] = useState<Session | null>(null);
  useEffect(() => {
    onBusyChange?.(busy);
  }, [busy, onBusyChange]);
  async function start() {
    if (busy || disabled || (beforeStart && !beforeStart())) return;
    setBusy(true);
    setError("");
    setActive(null);
    try {
      const response = await fetch("/api/sessions", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ template_id: templateID }),
      });
      const body = await response.json();
      if (!response.ok) {
        if (body.error?.code === "active_session_exists")
          setActive(body.error.session);
        else
          setError(
            body.error?.message ?? "Could not start workout. Please try again.",
          );
        return;
      }
      router.push(`/sessions/${body.id}`);
    } catch {
      setError(
        "Could not start workout. Please try again. Retrying will check for an active session.",
      );
    } finally {
      setBusy(false);
    }
  }
  return (
    <div
      className={compact ? "relative z-10 min-w-0 space-y-3" : "my-5 space-y-3"}
    >
      <Button disabled={busy || disabled} onClick={() => void start()}>
        {busy ? "Starting…" : "Start workout"}
      </Button>
      {active && (
        <Alert>
          <AlertTitle>A workout is already active</AlertTitle>
          <AlertDescription>
            Continue “{active.name}” before starting another workout.
          </AlertDescription>
          <Link
            href={`/sessions/${active.id}`}
            className="col-start-2 mt-3 inline-block font-semibold text-highlight underline"
          >
            Continue session
          </Link>
        </Alert>
      )}
      {error && (
        <Alert variant="destructive">
          <AlertTitle>Could not start workout</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
    </div>
  );
}
