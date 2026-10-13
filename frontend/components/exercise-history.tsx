"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import type { SavedSet } from "@/lib/sessions";

type ExerciseHistoryData = {
  id: string;
  name: string;
  workouts: {
    session_id: string;
    name: string;
    completed_at: string;
    occurrences: { position: number; name: string; sets: SavedSet[] }[];
  }[];
};
export function ExerciseHistory({ id }: { id: string }) {
  const [history, setHistory] = useState<ExerciseHistoryData | null>(null);
  const [loading, setLoading] = useState(true);
  const [missing, setMissing] = useState(false);
  const [error, setError] = useState("");
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setMissing(false);
    setError("");
    async function load() {
      try {
        const response = await fetch(
          `/api/exercises/${encodeURIComponent(id)}/history`,
          { cache: "no-store", signal: controller.signal },
        );
        if (response.status === 404) {
          if (!controller.signal.aborted) setMissing(true);
          return;
        }
        if (!response.ok) throw new Error();
        const body: ExerciseHistoryData = await response.json();
        if (!controller.signal.aborted) setHistory(body);
      } catch {
        if (!controller.signal.aborted)
          setError(
            "Could not load this exercise's completed sets. Please try again.",
          );
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    }
    void load();
    return () => controller.abort();
  }, [id, retry]);
  if (loading) return <p role="status">Loading exercise history…</p>;
  if (missing)
    return (
      <div className="space-y-3">
        <h1 className="text-3xl font-semibold">Exercise history not found</h1>
        <p className="text-muted-foreground">
          This exercise or its retained completed history could not be found.
        </p>
      </div>
    );
  if (error)
    return (
      <Alert variant="destructive">
        <AlertTitle>Could not open exercise history</AlertTitle>
        <AlertDescription>{error}</AlertDescription>
        <Button
          className="col-start-2 mt-3"
          onClick={() => setRetry(retry + 1)}
        >
          Try again
        </Button>
      </Alert>
    );
  if (!history) return null;
  return (
    <>
      <h1 className="break-words text-4xl font-semibold">{history.name}</h1>
      <p className="my-6 text-muted-foreground">
        Completed set history across all workouts, newest workouts first.
      </p>
      {!history.workouts.length ? (
        <p className="rounded-xl border border-dashed border-border p-5 text-muted-foreground">
          No completed sets for this exercise yet. Save sets in a workout and
          complete it to see them here.
        </p>
      ) : (
        <ol className="space-y-5">
          {history.workouts.map((workout) => (
            <li
              key={workout.session_id}
              className="rounded-2xl border border-border bg-card p-5"
            >
              <h2 className="break-words text-xl font-semibold">
                <Link
                  href={`/history/${workout.session_id}`}
                  className="text-highlight underline"
                >
                  {workout.name}
                </Link>
              </h2>
              <time
                className="mt-2 block text-sm text-muted-foreground"
                dateTime={workout.completed_at}
              >
                {new Date(workout.completed_at).toLocaleString()}
              </time>
              {workout.occurrences.map((occurrence) => (
                <section
                  key={occurrence.position}
                  aria-label={`Exercise occurrence ${occurrence.position} in ${workout.name}`}
                  className="mt-4 space-y-3"
                >
                  <h3 className="break-words font-medium">
                    Occurrence {occurrence.position}: {occurrence.name}
                  </h3>
                  <ol className="space-y-2 text-sm">
                    {occurrence.sets.map((set, index) => (
                      <li
                        key={set.id}
                        className="rounded-lg border border-border p-3"
                      >
                        Set {index + 1}: {set.weight} kg × {set.reps} reps
                      </li>
                    ))}
                  </ol>
                </section>
              ))}
            </li>
          ))}
        </ol>
      )}
    </>
  );
}
