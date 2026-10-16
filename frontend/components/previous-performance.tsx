"use client";
import { useEffect, useId, useRef, useState, type KeyboardEvent } from "react";
import { Button } from "@/components/ui/button";
import { RecordedSetsTable } from "@/components/recorded-sets-table";
import type { SavedSet } from "@/lib/sessions";

export type PreviousPerformance = {
  exercise_id: string;
  session_id: string;
  completed_at: string;
  occurrences: { position: number; name: string; sets: SavedSet[] }[];
};
type PersonalBest = PreviousPerformance;

// Reference requests never remount or reset the independent entry controls.
function useSessionReference<T>(
  sessionID: string,
  endpoint: "previous" | "bests",
  selectionKey: string,
) {
  const [data, setData] = useState<{ key: string; items: T[] }>({
    key: "",
    items: [],
  });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError(false);
    async function load() {
      try {
        const response = await fetch(
          `/api/sessions/${encodeURIComponent(sessionID)}/${endpoint}`,
          { cache: "no-store", signal: controller.signal },
        );
        if (!response.ok) throw new Error();
        const body = await response.json();
        if (!controller.signal.aborted)
          setData({ key: selectionKey, items: body.items });
      } catch {
        if (!controller.signal.aborted) setError(true);
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    }
    void load();
    return () => controller.abort();
  }, [sessionID, endpoint, attempt, selectionKey]);
  return {
    items: data.key === selectionKey ? data.items : [],
    loading: loading || (data.key !== selectionKey && !error),
    error,
    retry: () => setAttempt((value) => value + 1),
  };
}

export function usePreviousPerformance(sessionID: string, selectionKey = "") {
  return useSessionReference<PreviousPerformance>(
    sessionID,
    "previous",
    selectionKey,
  );
}
export function usePersonalBests(sessionID: string, selectionKey = "") {
  return useSessionReference<PersonalBest>(sessionID, "bests", selectionKey);
}

export function ExerciseReferences({
  label,
  previous,
  best,
  previousState,
  bestState,
}: {
  label: string;
  previous?: PreviousPerformance;
  best?: PersonalBest;
  previousState: { loading: boolean; error: boolean; retry: () => void };
  bestState: { loading: boolean; error: boolean; retry: () => void };
}) {
  const id = useId();
  const [selected, setSelected] = useState(0);
  const refs = useRef<(HTMLButtonElement | null)[]>([]);
  const tabs = ["Previous session", "Personal best"];
  function move(event: KeyboardEvent<HTMLButtonElement>) {
    let next: number;
    if (event.key === "ArrowRight" || event.key === "ArrowLeft")
      next = 1 - selected;
    else if (event.key === "Home") next = 0;
    else if (event.key === "End") next = 1;
    else return;
    event.preventDefault();
    setSelected(next);
    refs.current[next]?.focus();
  }
  const state = selected === 0 ? previousState : bestState;
  return (
    <section
      aria-label={`Reference sets for ${label}`}
      className="mt-5 space-y-3"
    >
      <div
        role="tablist"
        aria-label={`Reference views for ${label}`}
        className="flex flex-wrap gap-2"
      >
        {tabs.map((tab, index) => (
          <Button
            key={tab}
            ref={(node) => {
              refs.current[index] = node;
            }}
            type="button"
            role="tab"
            id={`${id}-tab-${index}`}
            aria-controls={`${id}-panel-${index}`}
            aria-selected={selected === index}
            tabIndex={selected === index ? 0 : -1}
            variant={selected === index ? "default" : "outline"}
            onClick={() => setSelected(index)}
            onKeyDown={move}
          >
            {tab}
          </Button>
        ))}
      </div>
      {tabs.map((_, index) => (
        <div
          key={index}
          role="tabpanel"
          id={`${id}-panel-${index}`}
          aria-labelledby={`${id}-tab-${index}`}
          hidden={selected !== index}
          tabIndex={0}
        >
          {selected === index &&
            (state.loading ? (
              <p role="status" className="text-sm text-muted-foreground">
                Loading reference sets…
              </p>
            ) : state.error ? (
              <div className="space-y-2">
                <p role="alert" className="text-sm text-destructive">
                  Could not load {tabs[index].toLowerCase()}.
                </p>
                <Button type="button" variant="outline" onClick={state.retry}>
                  Retry {tabs[index].toLowerCase()}
                </Button>
              </div>
            ) : (
              <ReferenceTables
                reference={index === 0 ? previous : best}
                label={label}
                kind={tabs[index]}
              />
            ))}
        </div>
      ))}
    </section>
  );
}

function ReferenceTables({
  reference,
  label,
  kind,
}: {
  reference?: PreviousPerformance;
  label: string;
  kind: string;
}) {
  if (!reference?.occurrences.length)
    return (
      <p className="text-sm text-muted-foreground">
        No {kind.toLowerCase()} yet.
      </p>
    );
  return reference.occurrences.map((occurrence, index) => (
    <div key={occurrence.position} className="not-first:mt-4">
      {reference.occurrences.length > 1 && (
        <p className="mb-1 text-sm text-muted-foreground">
          Occurrence {index + 1}
        </p>
      )}
      <RecordedSetsTable
        sets={occurrence.sets}
        caption={`${kind} sets for ${label}, occurrence ${index + 1}`}
      />
    </div>
  ));
}
