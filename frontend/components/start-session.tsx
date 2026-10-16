"use client";
import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import type { Session } from "@/lib/sessions";

export function StartSession({
  templateID,
  compact = false,
  prominent = false,
}: {
  templateID: string;
  compact?: boolean;
  prominent?: boolean;
}) {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [active, setActive] = useState<Session | null>(null);
  async function start() {
    if (busy) return;
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
      className={
        prominent
          ? "w-40 min-w-0 shrink-0 space-y-3 break-words sm:w-44"
          : compact
            ? "relative z-10 min-w-0 space-y-3"
            : "my-5 space-y-3"
      }
    >
      <Button
        size={prominent ? "lg" : "default"}
        className={
          prominent ? "min-h-12 w-full text-base font-semibold" : undefined
        }
        disabled={busy}
        onClick={() => void start()}
      >
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
