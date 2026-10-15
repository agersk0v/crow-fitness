"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import type { CompletedSet } from "@/lib/sessions";

export function CompletedSets({
  query = "",
  filtered = false,
}: {
  query?: string;
  filtered?: boolean;
}) {
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
        const response = await fetch(`/api/sets${query ? `?${query}` : ""}`, {
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
  }, [retry, query]);
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
  if (!items.length && filtered)
    return (
      <div className="space-y-3">
        <p className="text-muted-foreground">
          No matching completed sets. Try different criteria or clear filters.
        </p>
        <Link href="/history?tab=sets" className="text-highlight underline">
          Clear filters
        </Link>
      </div>
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
  const workouts: {
    id: string;
    name: string;
    completed_at: string;
    occurrences: { position: number; name: string; sets: CompletedSet[] }[];
  }[] = [];
  // The API has already filtered and ordered the rows. Preserve that order
  // and group by snapshot IDs/positions rather than mutable or repeated names.
  for (const set of items) {
    if (
      !workouts.length ||
      workouts[workouts.length - 1].id !== set.session_id
    ) {
      workouts.push({
        id: set.session_id,
        name: set.session_name,
        completed_at: set.completed_at,
        occurrences: [],
      });
    }
    const workout = workouts[workouts.length - 1];
    if (
      !workout.occurrences.length ||
      workout.occurrences[workout.occurrences.length - 1].position !==
        set.exercise_position
    ) {
      workout.occurrences.push({
        position: set.exercise_position,
        name: set.exercise_name,
        sets: [],
      });
    }
    workout.occurrences[workout.occurrences.length - 1].sets.push(set);
  }
  return (
    <ol className="space-y-5" aria-label="Completed sets by workout">
      {workouts.map((workout) => (
        <li
          key={workout.id}
          className="rounded-2xl border border-border bg-card p-5"
        >
          {workout.occurrences.map((occurrence) => (
            <section
              key={occurrence.position}
              aria-label={`Exercise occurrence ${occurrence.position} in ${workout.name}`}
              className="space-y-3 not-first:mt-5"
            >
              <h2 className="break-words text-xl font-semibold text-highlight">
                Occurrence {occurrence.position}: {occurrence.name}
              </h2>
              <ol className="space-y-2">
                {occurrence.sets.map((set, index) => (
                  <li
                    key={set.id}
                    className="rounded-lg border border-border bg-primary/5 p-3 font-medium tabular-nums"
                  >
                    Set {index + 1}: {set.weight} kg × {set.reps} reps
                  </li>
                ))}
              </ol>
            </section>
          ))}
          <div className="mt-5 border-t border-border pt-4">
            <p className="break-words text-sm text-muted-foreground">
              <Link
                href={`/history/${workout.id}`}
                className="underline focus-visible:outline-2 focus-visible:outline-ring"
              >
                {workout.name}
              </Link>
            </p>
            <time
              className="mt-2 block text-sm text-muted-foreground"
              dateTime={workout.completed_at}
            >
              {new Date(workout.completed_at).toLocaleString()}
            </time>
          </div>
        </li>
      ))}
    </ol>
  );
}
