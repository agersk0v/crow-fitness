"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import type { CompletedSet } from "@/lib/sessions";

export function CompletedSets() {
  const [items, setItems] = useState<CompletedSet[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    async function load() {
      try {
        const response = await fetch("/api/sets", {
          cache: "no-store",
          signal: controller.signal,
        });
        if (!response.ok) throw new Error();
        const body = await response.json();
        if (!controller.signal.aborted) setItems(body.items);
      } catch {
        if (!controller.signal.aborted)
          setError("Could not load completed sets. Please try again.");
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    }
    void load();
    return () => controller.abort();
  }, [retry]);
  if (loading) return <p role="status">Loading completed sets…</p>;
  if (error)
    return (
      <Alert variant="destructive">
        <AlertTitle>Could not load sets</AlertTitle>
        <AlertDescription>{error}</AlertDescription>
        <Button
          className="col-start-2 mt-3"
          onClick={() => setRetry(retry + 1)}
        >
          Try again
        </Button>
      </Alert>
    );
  if (!items.length)
    return (
      <div className="space-y-3">
        <p className="text-muted-foreground">
          No completed sets yet. Save sets in a workout and complete the session
          to see them here.
        </p>
        <Link href="/templates" className="text-highlight underline">
          Browse templates
        </Link>
      </div>
    );
  return (
    <ol className="space-y-3">
      {items.map((set) => (
        <li
          key={`${set.session_id}-${set.id}`}
          className="rounded-xl border border-border bg-card p-4"
        >
          <h2 className="break-words font-semibold">{set.exercise_name}</h2>
          <p className="mt-1">
            {set.weight} kg × {set.reps} reps
          </p>
          <time
            className="mt-2 block text-sm text-muted-foreground"
            dateTime={set.completed_at}
          >
            {new Date(set.completed_at).toLocaleString()}
          </time>
        </li>
      ))}
    </ol>
  );
}
