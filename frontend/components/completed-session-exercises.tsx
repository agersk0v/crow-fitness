"use client";

import { useId, useState } from "react";
import { ChevronDown } from "lucide-react";
import type { Session } from "@/lib/sessions";

export function CompletedSessionExercises({
  session,
  inline = false,
}: {
  session: Session;
  inline?: boolean;
}) {
  const prefix = useId();
  const [expanded, setExpanded] = useState<Set<number>>(() => new Set());
  if (inline)
    return (
      <ol className="divide-y divide-border">
        {session.exercises.map((entry) => {
          const sets = session.sets.filter(
            (set) => set.exercise_position === entry.position,
          );
          const open = expanded.has(entry.position);
          const trigger = `${prefix}-${entry.position}-trigger`;
          const panel = `${prefix}-${entry.position}-panel`;
          return (
            <li key={entry.position}>
              <h3>
                <button
                  type="button"
                  id={trigger}
                  aria-expanded={open}
                  aria-controls={panel}
                  className="flex min-h-12 w-full items-center gap-3 rounded-lg py-3 text-left hover:bg-primary/5 focus-visible:outline-2 focus-visible:outline-ring"
                  onClick={() =>
                    setExpanded((previous) => {
                      const next = new Set(previous);
                      if (open) next.delete(entry.position);
                      else next.add(entry.position);
                      return next;
                    })
                  }
                >
                  <span className="min-w-0 flex-1 break-words font-medium">
                    {entry.position}. {entry.name}
                  </span>
                  <span className="shrink-0 text-sm text-muted-foreground">
                    {sets.length} {sets.length === 1 ? "set" : "sets"}
                  </span>
                  <ChevronDown
                    aria-hidden="true"
                    className={`size-4 shrink-0 ${open ? "rotate-180" : ""}`}
                  />
                </button>
              </h3>
              <div
                id={panel}
                role="region"
                aria-labelledby={trigger}
                hidden={!open}
                className="space-y-3 pb-4"
              >
                {sets.length ? (
                  <ol className="space-y-2">
                    {sets.map((set, index) => (
                      <li
                        key={set.id}
                        className="rounded-lg bg-primary/5 px-3 py-2 font-medium tabular-nums"
                      >
                        Set {index + 1}: {set.weight} kg × {set.reps} reps
                      </li>
                    ))}
                  </ol>
                ) : (
                  <p className="text-sm text-muted-foreground">
                    No sets recorded for this exercise.
                  </p>
                )}
                <p className="text-sm text-muted-foreground">
                  Template targets: {entry.set_count} sets
                  {entry.rep_min !== null
                    ? ` · ${entry.rep_min === entry.rep_max ? entry.rep_min : `${entry.rep_min}–${entry.rep_max}`} reps`
                    : ""}
                  {entry.target_rir !== null
                    ? ` · ${entry.target_rir} RIR`
                    : ""}
                </p>
              </div>
            </li>
          );
        })}
      </ol>
    );
  const ExerciseHeading = "h2";
  const SectionHeading = "h3";
  return (
    <>
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
              <ExerciseHeading className="break-words text-xl font-semibold">
                {entry.position}. {entry.name}
              </ExerciseHeading>
              <section
                className="mt-4 rounded-xl border border-border p-4"
                aria-label={`Template targets for exercise ${entry.position}`}
              >
                <SectionHeading className="font-medium">
                  Template targets
                </SectionHeading>
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
                <SectionHeading className="font-medium">
                  Completed sets
                </SectionHeading>
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
