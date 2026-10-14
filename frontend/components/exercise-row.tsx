"use client";

import Link from "next/link";
import { useEffect, useId, useRef, useState, type FormEvent } from "react";
import { toast } from "sonner";
import { useEntranceAnimation } from "@/hooks/use-entrance-animation";
import { Button } from "@/components/ui/button";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import { MuscleGroupPicker } from "@/components/muscle-group-picker";
import {
  muscleGroupLabel,
  type Exercise,
  type TemplateReference,
} from "@/lib/exercises";

export function ExerciseRow({
  exercise,
  animateEntrance,
  groups,
  onUpdated,
  onDeleted,
}: {
  exercise: Exercise;
  animateEntrance: boolean;
  groups: string[];
  onUpdated: (exercise: Exercise) => void;
  onDeleted: (id: string) => void;
}) {
  const scope = useEntranceAnimation<HTMLLIElement>(animateEntrance);
  const dialog = useRef<HTMLDialogElement>(null);
  const id = useId();
  const [mode, setMode] = useState<"edit" | "delete" | null>(null);
  const [name, setName] = useState(exercise.name);
  const [tags, setTags] = useState(exercise.muscle_groups);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [fields, setFields] = useState<Record<string, string>>({});
  const [usage, setUsage] = useState<TemplateReference[]>([]);
  const [loadingUsage, setLoadingUsage] = useState(false);
  const [usageError, setUsageError] = useState("");
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    if (mode) dialog.current?.showModal();
    else dialog.current?.close();
  }, [mode]);
  useEffect(() => {
    if (mode !== "delete") return;
    const controller = new AbortController();
    setLoadingUsage(true);
    setUsageError("");
    setUsage([]);
    async function load() {
      try {
        const response = await fetch(
          `/api/exercises/${exercise.id}/templates`,
          { cache: "no-store", signal: controller.signal },
        );
        if (!response.ok) throw new Error("usage failed");
        const body = await response.json();
        if (!controller.signal.aborted) setUsage(body.items);
      } catch {
        if (!controller.signal.aborted)
          setUsageError("Could not check template usage. Please try again.");
      } finally {
        if (!controller.signal.aborted) setLoadingUsage(false);
      }
    }
    void load();
    return () => controller.abort();
  }, [mode, exercise.id, retry]);
  function open(next: "edit" | "delete") {
    setName(exercise.name);
    setTags(exercise.muscle_groups);
    setError("");
    setFields({});
    if (next === "delete") {
      setLoadingUsage(true);
      setUsage([]);
      setUsageError("");
    }
    setMode(next);
  }
  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    setBusy(true);
    setError("");
    setFields({});
    try {
      const response = await fetch(`/api/exercises/${exercise.id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name, muscle_groups: tags }),
      });
      const body = await response.json();
      if (!response.ok) {
        setError(
          body.error?.message ?? "Could not save exercise. Please try again.",
        );
        setFields(body.error?.fields ?? {});
        return;
      }
      setMode(null);
      onUpdated(body);
      toast.success(`${body.name} updated.`);
    } catch {
      setError("Could not save exercise. Please try again.");
    } finally {
      setBusy(false);
    }
  }
  async function remove() {
    if (busy || loadingUsage || usageError || usage.length) return;
    setBusy(true);
    setError("");
    try {
      const response = await fetch(`/api/exercises/${exercise.id}`, {
        method: "DELETE",
      });
      if (!response.ok) {
        const body = await response.json();
        setError(
          body.error?.message ?? "Could not delete exercise. Please try again.",
        );
        if (response.status === 409) setUsage(body.error?.templates ?? []);
        return;
      }
      setMode(null);
      onDeleted(exercise.id);
      toast.success(`${exercise.name} deleted.`);
    } catch {
      setError("Could not delete exercise. Please try again.");
    } finally {
      setBusy(false);
    }
  }
  return (
    <li ref={scope} className="relative min-w-0 px-5 py-4 hover:bg-primary/5">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <p className="break-words font-medium">
            <Link
              href={`/history?tab=sets&exercise=${exercise.id}`}
              aria-label={`View completed sets for ${exercise.name}`}
              className="text-highlight after:absolute after:inset-0 after:content-[''] hover:underline focus-visible:outline-none focus-visible:after:outline-2 focus-visible:after:outline-offset-[-2px] focus-visible:after:outline-ring"
            >
              {exercise.name}
            </Link>
          </p>
          <div className="mt-2 flex flex-wrap gap-2">
            {exercise.muscle_groups.map((group) => (
              <span
                key={group}
                className="rounded-full bg-primary/10 px-2 py-1 text-xs text-highlight"
              >
                {muscleGroupLabel(group)}
              </span>
            ))}
          </div>
        </div>
        <div className="flex w-full flex-wrap gap-2 sm:w-auto">
          <Button
            className="relative z-10"
            variant="outline"
            onClick={() => open("edit")}
            aria-label={`Edit ${exercise.name}`}
          >
            Edit
          </Button>
          <Button
            className="relative z-10"
            variant="outline"
            onClick={() => open("delete")}
            aria-label={`Delete ${exercise.name}`}
          >
            Delete
          </Button>
        </div>
      </div>
      <dialog
        ref={dialog}
        aria-labelledby={`${id}-title`}
        aria-describedby={mode === "delete" ? `${id}-description` : undefined}
        onCancel={(event) => {
          event.preventDefault();
          if (!busy) setMode(null);
        }}
        onClose={() => setMode(null)}
        className="m-auto max-h-[90dvh] w-[calc(100%-2rem)] max-w-lg overflow-y-auto rounded-2xl border border-border bg-card p-5 text-foreground shadow-xl backdrop:bg-black/60"
      >
        <h2 id={`${id}-title`} className="break-words text-xl font-semibold">
          {mode === "edit" ? "Edit exercise" : `Delete ${exercise.name}?`}
        </h2>
        {mode === "edit" ? (
          <form onSubmit={save} noValidate className="mt-5 space-y-4">
            <fieldset disabled={busy} className="space-y-4">
              <label
                htmlFor={`${id}-name`}
                className="block text-sm font-medium"
              >
                Exercise name
              </label>
              <input
                autoFocus
                id={`${id}-name`}
                value={name}
                onChange={(event) => setName(event.target.value)}
                aria-invalid={Boolean(fields.name)}
                aria-describedby={fields.name ? `${id}-name-error` : undefined}
                className="min-h-12 w-full rounded-lg border border-input bg-background px-3 text-base focus-visible:outline-2 focus-visible:outline-ring"
              />
              {fields.name && (
                <p id={`${id}-name-error`} className="text-sm text-destructive">
                  {fields.name}
                </p>
              )}
              <MuscleGroupPicker
                groups={groups}
                selected={tags}
                onChange={setTags}
                error={fields.muscle_groups}
              />
            </fieldset>
            {error && (
              <Alert variant="destructive">
                <AlertTitle>Could not save exercise</AlertTitle>
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}
            <div className="flex flex-wrap gap-3">
              <Button type="submit" disabled={busy}>
                {busy ? "Saving…" : "Save"}
              </Button>
              <Button
                type="button"
                variant="outline"
                disabled={busy}
                onClick={() => setMode(null)}
              >
                Cancel
              </Button>
            </div>
          </form>
        ) : (
          <div className="mt-4 space-y-4">
            <p
              id={`${id}-description`}
              className="break-words text-sm text-muted-foreground"
            >
              Delete “{exercise.name}” from your library? This cannot be undone.
            </p>
            {loadingUsage && <p role="status">Checking template usage…</p>}
            {usageError && (
              <Alert variant="destructive">
                <AlertTitle>Could not check usage</AlertTitle>
                <AlertDescription>{usageError}</AlertDescription>
                <Button className="mt-3" onClick={() => setRetry(retry + 1)}>
                  Try again
                </Button>
              </Alert>
            )}
            {usage.length > 0 && (
              <div>
                <p className="text-sm font-medium">
                  Cannot delete: used in {usage.length}{" "}
                  {usage.length === 1 ? "template" : "templates"}.
                </p>
                <ul className="mt-2 space-y-2">
                  {usage.map((template) => (
                    <li key={template.id}>
                      <Link
                        href={`/templates/${template.id}`}
                        className="break-words text-highlight underline"
                      >
                        {template.name}
                        {template.archived ? " (Archived)" : ""}
                      </Link>
                    </li>
                  ))}
                </ul>
              </div>
            )}
            {error && (
              <Alert variant="destructive">
                <AlertTitle>Could not delete exercise</AlertTitle>
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}
            <div className="flex flex-wrap gap-3">
              <Button
                variant="outline"
                disabled={busy}
                autoFocus
                onClick={() => setMode(null)}
              >
                Cancel
              </Button>
              <Button
                variant="destructive"
                disabled={
                  busy ||
                  loadingUsage ||
                  Boolean(usageError) ||
                  usage.length > 0
                }
                onClick={() => void remove()}
              >
                {busy ? "Deleting…" : "Delete exercise"}
              </Button>
            </div>
          </div>
        )}
      </dialog>
    </li>
  );
}
