"use client";
import Link from "next/link";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { StartSession } from "@/components/start-session";
import type { Template, TemplateSummary } from "@/lib/templates";

export function TemplateCard({ template }: { template: TemplateSummary }) {
  const [details, setDetails] = useState<Template | null>(null);
  const [error, setError] = useState(false);
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setDetails(null);
    setError(false);
    async function load() {
      try {
        const response = await fetch(
          `/api/templates/${encodeURIComponent(template.id)}`,
          {
            cache: "no-store",
            signal: controller.signal,
          },
        );
        if (!response.ok) throw new Error();
        const body: Template = await response.json();
        if (!controller.signal.aborted) setDetails(body);
      } catch {
        if (!controller.signal.aborted) setError(true);
      }
    }
    void load();
    return () => controller.abort();
  }, [template.id, retry]);
  const current = details ?? template;
  return (
    <li className="min-w-0 rounded-2xl border border-border bg-card p-5 sm:p-6">
      <div className="flex items-start justify-between gap-3">
        <h3 className="min-w-0 flex-1 break-words text-xl font-semibold text-highlight">
          {current.name}
          {current.archived ? " (Archived)" : ""}
        </h3>
        {!current.archived && (
          <StartSession templateID={template.id} prominent />
        )}
      </div>
      {current.description && (
        <p className="mt-2 whitespace-pre-wrap break-words text-sm text-muted-foreground">
          {current.description}
        </p>
      )}
      <div className="my-5">
        {error ? (
          <Alert variant="destructive">
            <AlertTitle>Could not load exercises</AlertTitle>
            <AlertDescription>
              Please try again to see this workout’s exercises.
            </AlertDescription>
            <Button
              className="col-start-2 mt-3"
              variant="outline"
              onClick={() => setRetry((value) => value + 1)}
            >
              Try again
            </Button>
          </Alert>
        ) : !details ? (
          <p role="status" className="text-sm text-muted-foreground">
            Loading exercises…
          </p>
        ) : (
          <ol className="space-y-3" aria-label={`Exercises in ${current.name}`}>
            {details.exercises.map((exercise) => (
              <li
                key={exercise.position}
                className="min-w-0 break-words text-sm"
              >
                <span className="font-medium">
                  {exercise.position}. {exercise.name}
                </span>
                <span className="text-muted-foreground">
                  {" "}
                  · {exercise.set_count}{" "}
                  {exercise.set_count === 1 ? "set" : "sets"}
                  {exercise.rep_min !== null && (
                    <>
                      {" "}
                      ·{" "}
                      {exercise.rep_min === exercise.rep_max
                        ? exercise.rep_min
                        : `${exercise.rep_min}–${exercise.rep_max}`}{" "}
                      reps
                    </>
                  )}
                  {exercise.target_rir !== null && (
                    <> · RIR {exercise.target_rir}</>
                  )}
                </span>
                {!!exercise.alternatives?.length && (
                  <div className="mt-1 space-y-1 text-muted-foreground">
                    <p>Default: {exercise.name}</p>
                    {exercise.alternatives.map((c) => (
                      <p key={c.exercise_id}>
                        Or {c.name} · {c.set_count} sets
                        {c.rep_min !== null &&
                          ` · ${c.rep_min}–${c.rep_max} reps`}
                        {c.target_rir !== null && ` · RIR ${c.target_rir}`}
                      </p>
                    ))}
                  </div>
                )}
              </li>
            ))}
          </ol>
        )}
      </div>
      <div className="flex flex-wrap items-start gap-3">
        <Button asChild variant="outline">
          <Link href={`/templates/${template.id}`}>
            {current.archived ? "View workout" : "Edit workout"}
          </Link>
        </Button>
      </div>
    </li>
  );
}
