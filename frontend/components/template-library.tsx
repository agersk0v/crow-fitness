"use client";
import Link from "next/link";
import { useEffect, useState, type FormEvent } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import type { Exercise } from "@/lib/static-exercises";
import type { TemplateSummary } from "@/lib/templates";

type DraftEntry = {
  key: string;
  exercise_id: string;
  set_count: string;
  rep_min: string;
  rep_max: string;
  target_rir: string;
};
const inputClass =
  "min-h-12 w-full rounded-lg border border-input bg-background px-3 text-base focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring";
export function TemplateLibrary() {
  const [templates, setTemplates] = useState<TemplateSummary[]>([]);
  const [exercises, setExercises] = useState<Exercise[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [retry, setRetry] = useState(0);
  const [name, setName] = useState("");
  const [entries, setEntries] = useState<DraftEntry[]>([]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [fields, setFields] = useState<Record<string, string>>({});
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setLoadError("");
    async function load() {
      try {
        const [list, library] = await Promise.all([
          fetch("/api/templates", {
            cache: "no-store",
            signal: controller.signal,
          }),
          fetch("/api/exercises", {
            cache: "no-store",
            signal: controller.signal,
          }),
        ]);
        if (!list.ok || !library.ok) throw new Error("load failed");
        const [listBody, libraryBody] = await Promise.all([
          list.json(),
          library.json(),
        ]);
        if (!controller.signal.aborted) {
          setTemplates(listBody.items);
          setExercises(libraryBody.items);
        }
      } catch {
        if (!controller.signal.aborted)
          setLoadError(
            "Could not load templates and exercises. Please try again.",
          );
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    }
    void load();
    return () => controller.abort();
  }, [retry]);
  function update(key: string, field: keyof DraftEntry, value: string) {
    setEntries((current) =>
      current.map((entry) =>
        entry.key === key ? { ...entry, [field]: value } : entry,
      ),
    );
  }
  function move(index: number, direction: number) {
    setEntries((current) => {
      const next = [...current];
      [next[index], next[index + direction]] = [
        next[index + direction],
        next[index],
      ];
      return next;
    });
  }
  function fieldError(field: string) {
    return fields[field] ? (
      <p id={`error-${field}`} className="text-sm text-destructive">
        {fields[field]}
      </p>
    ) : null;
  }
  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (saving) return;
    setSaving(true);
    setError("");
    setFields({});
    // Keep blank targets null; Go validates business rules and numeric values.
    const numeric = (value: string) => (value === "" ? null : Number(value));
    try {
      const response = await fetch("/api/templates", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name,
          exercises: entries.map((entry) => ({
            exercise_id: entry.exercise_id,
            set_count: numeric(entry.set_count),
            rep_min: numeric(entry.rep_min),
            rep_max: numeric(entry.rep_max),
            target_rir: numeric(entry.target_rir),
          })),
        }),
      });
      const body = await response.json();
      if (!response.ok) {
        setError(
          body.error?.message ?? "Could not save template. Please try again.",
        );
        setFields(body.error?.fields ?? {});
        return;
      }
      setTemplates((current) => [...current, { id: body.id, name: body.name }]);
      setName("");
      setEntries([]);
      toast.success(`${body.name} saved.`);
    } catch {
      setError("Could not save template. Please try again.");
    } finally {
      setSaving(false);
    }
  }
  return (
    <div className="space-y-8">
      {loadError && (
        <Alert variant="destructive">
          <AlertTitle>Could not load templates</AlertTitle>
          <AlertDescription>{loadError}</AlertDescription>
          <Button className="mt-3" onClick={() => setRetry(retry + 1)}>
            Try again
          </Button>
        </Alert>
      )}
      <section aria-labelledby="saved-templates">
        <h2 id="saved-templates" className="mb-4 text-xl font-semibold">
          Saved templates
        </h2>
        {loading ? (
          <p role="status">Loading templates…</p>
        ) : (
          !loadError &&
          (templates.length === 0 ? (
            <p className="rounded-2xl border border-dashed border-border p-6 text-muted-foreground">
              No templates yet. Create your first routine below.
            </p>
          ) : (
            <ul className="divide-y divide-border rounded-2xl border border-border bg-card">
              {templates.map((template) => (
                <li key={template.id}>
                  <Link
                    href={`/templates/${template.id}`}
                    className="block break-words px-5 py-4 font-medium text-highlight hover:bg-primary/5"
                  >
                    {template.name} →
                  </Link>
                </li>
              ))}
            </ul>
          ))
        )}
      </section>
      <section
        aria-labelledby="create-template"
        className="rounded-2xl border border-border bg-card p-5 sm:p-6"
      >
        <h2 id="create-template" className="text-xl font-semibold">
          Create a template
        </h2>
        <p className="mt-2 text-sm text-muted-foreground">
          Set counts cover working sets only. Optional rep and RIR targets apply
          to every working set.
        </p>
        {!loading && !loadError && exercises.length === 0 && (
          <p className="mt-4 text-muted-foreground">
            <Link href="/exercises" className="text-highlight underline">
              Add an exercise
            </Link>{" "}
            to your library before creating a template.
          </p>
        )}
        <form onSubmit={save} noValidate className="mt-5 space-y-5">
          <fieldset disabled={saving} className="min-w-0 space-y-5">
            <div>
              <label
                htmlFor="template-name"
                className="mb-2 block text-sm font-medium"
              >
                Template name
              </label>
              <input
                id="template-name"
                value={name}
                onChange={(event) => setName(event.target.value)}
                className={inputClass}
                aria-invalid={Boolean(fields.name)}
                aria-describedby={fields.name ? "error-name" : undefined}
                placeholder="e.g. Push day"
              />
              {fieldError("name")}
            </div>
            {entries.map((entry, index) => {
              const prefix = `exercises.${index}.`;
              const label =
                exercises.find((exercise) => exercise.id === entry.exercise_id)
                  ?.name ?? `Exercise ${index + 1}`;
              return (
                <fieldset
                  key={entry.key}
                  className="min-w-0 rounded-xl border border-border p-4"
                >
                  <legend className="px-2 text-sm font-semibold">
                    Exercise {index + 1}
                  </legend>
                  <label
                    htmlFor={`${entry.key}-exercise`}
                    className="mb-2 block text-sm font-medium"
                  >
                    Exercise
                  </label>
                  <select
                    id={`${entry.key}-exercise`}
                    value={entry.exercise_id}
                    onChange={(event) =>
                      update(entry.key, "exercise_id", event.target.value)
                    }
                    className={inputClass}
                    aria-invalid={Boolean(fields[prefix + "exercise_id"])}
                    aria-describedby={
                      fields[prefix + "exercise_id"]
                        ? `error-${prefix}exercise_id`
                        : undefined
                    }
                  >
                    <option value="">Choose an exercise</option>
                    {exercises.map((exercise) => (
                      <option key={exercise.id} value={exercise.id}>
                        {exercise.name}
                      </option>
                    ))}
                  </select>
                  {fieldError(prefix + "exercise_id")}
                  <div className="mt-4 grid gap-4 sm:grid-cols-2">
                    {(
                      [
                        ["set_count", "Working-set count"],
                        ["rep_min", "Minimum reps (optional)"],
                        ["rep_max", "Maximum reps (optional)"],
                        ["target_rir", "Target RIR (optional)"],
                      ] as const
                    ).map(([field, title]) => {
                      const errorKey =
                        prefix +
                        (field.startsWith("rep_") ? "rep_range" : field);
                      return (
                        <div key={field}>
                          <label
                            htmlFor={`${entry.key}-${field}`}
                            className="mb-2 block text-sm font-medium"
                          >
                            {title}
                          </label>
                          <input
                            id={`${entry.key}-${field}`}
                            type="number"
                            step={field === "target_rir" ? "any" : "1"}
                            value={entry[field]}
                            onChange={(event) =>
                              update(entry.key, field, event.target.value)
                            }
                            className={inputClass}
                            aria-invalid={Boolean(fields[errorKey])}
                            aria-describedby={
                              fields[errorKey] ? `error-${errorKey}` : undefined
                            }
                          />
                          {field.startsWith("rep_")
                            ? null
                            : fieldError(errorKey)}
                        </div>
                      );
                    })}
                  </div>
                  {fieldError(prefix + "rep_range")}
                  <div className="mt-4 flex flex-wrap gap-2">
                    <Button
                      type="button"
                      variant="outline"
                      disabled={index === 0}
                      aria-label={`Move ${label} up`}
                      onClick={() => move(index, -1)}
                    >
                      Move up
                    </Button>
                    <Button
                      type="button"
                      variant="outline"
                      disabled={index === entries.length - 1}
                      aria-label={`Move ${label} down`}
                      onClick={() => move(index, 1)}
                    >
                      Move down
                    </Button>
                    <Button
                      type="button"
                      variant="outline"
                      aria-label={`Remove ${label} from draft`}
                      onClick={() =>
                        setEntries((current) =>
                          current.filter((item) => item.key !== entry.key),
                        )
                      }
                    >
                      Remove
                    </Button>
                  </div>
                </fieldset>
              );
            })}
            {fieldError("exercises")}
            <Button
              type="button"
              variant="outline"
              disabled={loading || Boolean(loadError) || exercises.length === 0}
              onClick={() =>
                setEntries((current) => [
                  ...current,
                  {
                    key: crypto.randomUUID(),
                    exercise_id: "",
                    set_count: "",
                    rep_min: "",
                    rep_max: "",
                    target_rir: "",
                  },
                ])
              }
            >
              Add exercise to template
            </Button>
          </fieldset>
          {error && (
            <Alert variant="destructive">
              <AlertTitle>Could not save template</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}
          <Button
            type="submit"
            disabled={
              saving || loading || Boolean(loadError) || exercises.length === 0
            }
            className="min-h-12 w-full sm:w-auto"
          >
            {saving ? "Saving…" : "Save template"}
          </Button>
        </form>
      </section>
    </div>
  );
}
