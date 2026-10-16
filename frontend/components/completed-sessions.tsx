"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { CompletedSessionDetail } from "@/components/completed-session-detail";
import { Button } from "@/components/ui/button";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import type { CompletedSession } from "@/lib/sessions";

export function CompletedSessions({
  query = "",
  filtered = false,
}: {
  query?: string;
  filtered?: boolean;
}) {
  const [items, setItems] = useState<CompletedSession[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    async function load() {
      try {
        const response = await fetch(
          `/api/history/sessions${query ? `?${query}` : ""}`,
          {
            cache: "no-store",
            signal: controller.signal,
          },
        );
        if (!response.ok) throw new Error();
        const body = await response.json();
        if (!controller.signal.aborted) setItems(body.items);
      } catch {
        if (!controller.signal.aborted)
          setError("Could not load completed sessions. Please try again.");
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    }
    void load();
    return () => controller.abort();
  }, [retry, query]);
  if (loading) return <p role="status">Loading completed sessions…</p>;
  if (error)
    return (
      <Alert variant="destructive">
        <AlertTitle>Could not load sessions</AlertTitle>
        <AlertDescription>{error}</AlertDescription>
        <Button
          className="col-start-2 mt-3"
          onClick={() => setRetry(retry + 1)}
        >
          Try again
        </Button>
      </Alert>
    );
  if (!items.length && filtered)
    return (
      <div className="space-y-3">
        <p className="text-muted-foreground">
          No matching completed sessions. Try different criteria or clear
          filters.
        </p>
        <Link href="/history?tab=sessions" className="text-highlight underline">
          Clear filters
        </Link>
      </div>
    );
  if (!items.length)
    return (
      <div className="space-y-3">
        <p className="text-muted-foreground">
          No completed sessions yet. Complete a workout to keep it here.
        </p>
        <Link href="/templates" className="text-highlight underline">
          Browse templates
        </Link>
      </div>
    );
  return (
    <ul className="space-y-4" aria-label="Completed workouts">
      {items.map((session) => (
        <li
          key={session.id}
          className="rounded-2xl border border-border bg-card p-5"
        >
          <div className="mb-4 flex flex-wrap items-baseline justify-between gap-x-4 gap-y-2">
            <h2 className="min-w-0 break-words text-xl font-semibold text-highlight">
              {session.name}
            </h2>
            <time
              className="text-sm text-muted-foreground"
              dateTime={session.completed_at}
            >
              {new Date(session.completed_at).toLocaleString()}
            </time>
          </div>
          <CompletedSessionDetail id={session.id} inline />
        </li>
      ))}
    </ul>
  );
}
