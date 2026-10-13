"use client";
import Link from "next/link";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import type { Session } from "@/lib/sessions";

export function CompletedSessionDetail({ id }: { id: string }) {
  const [session, setSession] = useState<Session | null>(null);
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
          `/api/history/sessions/${encodeURIComponent(id)}`,
          { cache: "no-store", signal: controller.signal },
        );
        if (response.status === 404) {
          if (!controller.signal.aborted) setMissing(true);
          return;
        }
        if (!response.ok) throw new Error();
        const body: Session = await response.json();
        if (!controller.signal.aborted) setSession(body);
      } catch {
        if (!controller.signal.aborted)
          setError("Could not load this completed session. Please try again.");
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    }
    void load();
    return () => controller.abort();
  }, [id, retry]);
  if (loading) return <p role="status">Loading completed session…</p>;
  if (missing)
    return (
      <div className="space-y-3">
        <h1 className="text-3xl font-semibold">Completed session not found</h1>
        <p className="text-muted-foreground">
          This completed workout could not be found.
        </p>
      </div>
    );
  if (error)
    return (
      <Alert variant="destructive">
        <AlertTitle>Could not open completed session</AlertTitle>
        <AlertDescription>{error}</AlertDescription>
        <Button
          className="col-start-2 mt-3"
          onClick={() => setRetry(retry + 1)}
        >
          Try again
        </Button>
      </Alert>
    );
  if (!session || !session.completed_at) return null;
  return (
    <>
      <p className="text-sm font-medium text-highlight">Completed workout</p>
      <h1 className="mt-2 break-words text-4xl font-semibold">
        {session.name}
      </h1>
      <time
        className="mt-3 block text-sm text-muted-foreground"
        dateTime={session.completed_at}
      >
        {new Date(session.completed_at).toLocaleString()}
      </time>
      <p className="my-6 text-muted-foreground">
        This is your saved workout. Template targets are separate from the sets
        you actually recorded.
      </p>
      {!session.sets.length && (
        <p className="mb-6 rounded-xl border border-dashed border-border p-4 text-muted-foreground">
          No sets recorded in this workout.
        </p>
      )}
      <ol className="space-y-4">
        {session.exercises.map((entry) => {
          const sets = session.sets.filter(
            (set) => set.exercise_position === entry.position,
          );
          return (
            <li
              key={entry.position}
              className="rounded-2xl border border-border bg-card p-5"
            >
              <h2 className="break-words text-xl font-semibold">
                {entry.position}. {entry.name}
              </h2>
              <Link
                href={`/exercises/${entry.exercise_id}/history`}
                className="mt-2 inline-block text-sm text-highlight underline"
                aria-label={`View history for occurrence ${entry.position}: ${entry.name}`}
              >
                View exercise history
              </Link>
              <section
                className="mt-4 rounded-xl border border-border p-4"
                aria-label={`Template targets for exercise ${entry.position}`}
              >
                <h3 className="font-medium">Template targets</h3>
                <dl className="mt-3 grid grid-cols-2 gap-3 text-sm">
                  <dt>Working sets</dt>
                  <dd>{entry.set_count}</dd>
                  <dt>Rep target</dt>
                  <dd>
                    {entry.rep_min === null
                      ? "Not specified"
                      : entry.rep_min === entry.rep_max
                        ? entry.rep_min
                        : `${entry.rep_min}–${entry.rep_max}`}
                  </dd>
                  <dt>Target RIR</dt>
                  <dd>{entry.target_rir ?? "Not specified"}</dd>
                </dl>
              </section>
              <section
                className="mt-4"
                aria-label={`Completed sets for exercise ${entry.position}`}
              >
                <h3 className="font-medium">Completed sets</h3>
                {sets.length ? (
                  <ol className="mt-3 space-y-2 text-sm">
                    {sets.map((set, index) => (
                      <li
                        key={set.id}
                        className="rounded-lg border border-border p-3"
                      >
                        Set {index + 1}: {set.weight} kg × {set.reps} reps
                      </li>
                    ))}
                  </ol>
                ) : (
                  <p className="mt-2 text-sm text-muted-foreground">
                    No sets recorded for this exercise.
                  </p>
                )}
              </section>
            </li>
          );
        })}
      </ol>
    </>
  );
}
