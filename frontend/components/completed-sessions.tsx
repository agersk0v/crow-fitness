"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
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
        <Link href="/history" className="text-highlight underline">
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
    <ul className="grid gap-4 sm:grid-cols-2">
      {items.map((session) => (
        <li key={session.id}>
          <Link
            href={`/history/${session.id}`}
            className="block h-full rounded-2xl border border-border bg-card p-5 hover:bg-primary/5 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
          >
            <h2 className="break-words text-xl font-semibold text-highlight">
              {session.name} →
            </h2>
            <time
              className="mt-3 block text-sm text-muted-foreground"
              dateTime={session.completed_at}
            >
              {new Date(session.completed_at).toLocaleString()}
            </time>
          </Link>
        </li>
      ))}
    </ul>
  );
}
