"use client";
import { useEffect, useState } from "react";
import type { SavedSet } from "@/lib/sessions";

export type PreviousPerformance = {
  exercise_id: string;
  session_id: string;
  completed_at: string;
  occurrences: { position: number; name: string; sets: SavedSet[] }[];
};

// Reference loading is independent of current-session loading and local set drafts.
export function usePreviousPerformance(sessionID: string) {
  const [items, setItems] = useState<PreviousPerformance[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    async function load() {
      try {
        const response = await fetch(
          `/api/sessions/${encodeURIComponent(sessionID)}/previous`,
          { cache: "no-store", signal: controller.signal },
        );
        if (!response.ok)
          throw new Error("Could not load previous performance.");
        const body = await response.json();
        if (!controller.signal.aborted) setItems(body.items);
      } catch {
        if (!controller.signal.aborted)
          setError(
            "Previous results could not be loaded. Current set entry is still available.",
          );
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    }
    void load();
    return () => controller.abort();
  }, [sessionID, attempt]);
  return {
    items,
    loading,
    error,
    retry: () => setAttempt((current) => current + 1),
  };
}

export function PreviousPerformanceView({
  reference,
  loading,
  failed,
  label,
}: {
  reference: PreviousPerformance | undefined;
  loading: boolean;
  failed: boolean;
  label: string;
}) {
  return (
    <section
      aria-label={`Previous performance for ${label}`}
      className="mt-5 space-y-3 rounded-xl border border-border bg-background p-4"
    >
      <h3 className="font-medium">Previous performance</h3>
      {loading ? (
        <p role="status" className="text-sm text-muted-foreground">
          Loading previous performance…
        </p>
      ) : failed ? (
        <p className="text-sm text-muted-foreground">
          Previous performance is unavailable. You can continue saving current
          sets.
        </p>
      ) : !reference ? (
        <p className="text-sm text-muted-foreground">
          No previous performance yet.
        </p>
      ) : (
        <>
          <time
            dateTime={reference.completed_at}
            className="block text-sm text-muted-foreground"
          >
            {new Date(reference.completed_at).toLocaleString()}
          </time>
          <p className="text-xs text-muted-foreground">
            Completed workout results — separate from your current sets.
          </p>
          {reference.occurrences.map((occurrence) => (
            <div key={occurrence.position} className="space-y-2">
              <h4 className="break-words text-sm font-medium">
                Previous occurrence {occurrence.position}: {occurrence.name}
              </h4>
              <ol className="space-y-1 text-sm">
                {occurrence.sets.map((set, index) => (
                  <li key={set.id}>
                    Set {index + 1}: {set.weight} kg × {set.reps} reps
                  </li>
                ))}
              </ol>
            </div>
          ))}
        </>
      )}
    </section>
  );
}
