import type { Session } from "@/lib/sessions";

export function CompletedSessionExercises({
  session,
  inline = false,
}: {
  session: Session;
  inline?: boolean;
}) {
  const ExerciseHeading = inline ? "h3" : "h2";
  const SectionHeading = inline ? "h4" : "h3";
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
