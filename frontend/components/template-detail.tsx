"use client";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import type { Template } from "@/lib/templates";

export function TemplateDetail({ id }: { id: string }) {
  const [template, setTemplate] = useState<Template | null>(null);
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
          `/api/templates/${encodeURIComponent(id)}`,
          { cache: "no-store", signal: controller.signal },
        );
        if (!response.ok)
          throw new Error(
            response.status === 404
              ? "This template could not be found."
              : "Could not load this template. Please try again.",
          );
        const body: Template = await response.json();
        if (!controller.signal.aborted) setTemplate(body);
      } catch (e) {
        if (!controller.signal.aborted)
          setError(
            e instanceof Error ? e.message : "Could not load this template.",
          );
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    }
    void load();
    return () => controller.abort();
  }, [id, retry]);
  if (loading) return <p role="status">Loading template…</p>;
  if (error)
    return (
      <Alert variant="destructive">
        <AlertTitle>Could not open template</AlertTitle>
        <AlertDescription>{error}</AlertDescription>
        <Button className="mt-3" onClick={() => setRetry(retry + 1)}>
          Try again
        </Button>
      </Alert>
    );
  if (!template) return null;
  return (
    <>
      <h1 className="break-words text-4xl font-semibold">{template.name}</h1>
      <p className="mb-6 mt-3 text-muted-foreground">
        Targets apply to every working set of each exercise. Warm-up sets are
        excluded.
      </p>
      <ol className="space-y-4">
        {template.exercises.map((entry) => (
          <li
            key={entry.position}
            className="rounded-2xl border border-border bg-card p-5"
          >
            <h2 className="break-words text-xl font-semibold">
              {entry.position}. {entry.name}
            </h2>
            <dl className="mt-4 grid grid-cols-2 gap-3 text-sm">
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
          </li>
        ))}
      </ol>
    </>
  );
}
