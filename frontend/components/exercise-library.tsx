"use client";

import { useEffect, useState, type FormEvent } from "react";
import { toast } from "sonner";
import { ExerciseRow } from "@/components/exercise-row";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import type { Exercise } from "@/lib/static-exercises";

export function ExerciseLibrary() {
  const [exercises, setExercises] = useState<Exercise[]>([]);
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const [listError, setListError] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [addedIDs, setAddedIDs] = useState<string[]>([]);

  async function loadExercises(signal?: AbortSignal) {
    setLoading(true);
    setListError("");
    try {
      const response = await fetch("/api/exercises", {
        cache: "no-store",
        signal,
      });
      if (!response.ok)
        throw new Error("Could not load exercises. Please try again.");
      const body = await response.json();
      if (!signal?.aborted) setExercises(body.items);
    } catch {
      if (!signal?.aborted)
        setListError("Could not load exercises. Please try again.");
    } finally {
      if (!signal?.aborted) setLoading(false);
    }
  }
  useEffect(() => {
    const controller = new AbortController();
    void loadExercises(controller.signal);
    return () => controller.abort();
  }, []);

  async function addExercise(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (saving) return;
    setSaving(true);
    setError("");
    try {
      const response = await fetch("/api/exercises", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name }),
      });
      const body = await response.json();
      if (!response.ok) {
        setError(
          body.error?.fields?.name ??
            body.error?.message ??
            "Could not add exercise. Please try again.",
        );
        return;
      }
      setExercises((current) => [...current, body]);
      setAddedIDs((current) => [...current, body.id]);
      setName("");
      toast.success(`${body.name} added.`);
      await loadExercises();
    } catch {
      setError("Could not add exercise. Please try again.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="grid gap-8 lg:grid-cols-[1fr_20rem]">
      <section aria-labelledby="exercise-list-heading" className="min-w-0">
        <h2 id="exercise-list-heading" className="mb-4 text-xl font-semibold">
          Exercise library
        </h2>
        {loading && exercises.length === 0 ? (
          <p role="status">Loading exercises…</p>
        ) : listError ? (
          <Alert variant="destructive">
            <AlertTitle>Could not load exercises</AlertTitle>
            <AlertDescription>{listError}</AlertDescription>
            <Button onClick={() => void loadExercises()} className="mt-3">
              Try again
            </Button>
          </Alert>
        ) : exercises.length === 0 ? (
          <p className="rounded-2xl border border-dashed border-border p-6 text-muted-foreground">
            No exercises yet. Add your first exercise using the form.
          </p>
        ) : (
          <ul className="divide-y divide-border rounded-2xl border border-border bg-card">
            {exercises.map((exercise) => (
              <ExerciseRow
                key={exercise.id}
                exercise={exercise}
                animateEntrance={addedIDs.includes(exercise.id)}
              />
            ))}
          </ul>
        )}
      </section>
      <section
        aria-labelledby="add-exercise-heading"
        className="self-start rounded-2xl border border-border bg-card p-5"
      >
        <h2 id="add-exercise-heading" className="text-xl font-semibold">
          Add an exercise
        </h2>
        <form
          onSubmit={addExercise}
          noValidate
          className="mt-5 flex flex-col gap-3"
        >
          <label htmlFor="exercise-name" className="text-sm font-medium">
            Exercise name
          </label>
          <input
            id="exercise-name"
            name="name"
            type="text"
            value={name}
            disabled={saving}
            onChange={(event) => setName(event.target.value)}
            aria-invalid={Boolean(error)}
            aria-describedby={
              error ? "exercise-help exercise-error" : "exercise-help"
            }
            className="min-h-12 w-full rounded-lg border border-input bg-background px-3 text-base focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
            placeholder="e.g. Overhead press"
          />
          <p id="exercise-help" className="text-sm text-muted-foreground">
            Choose a name you will recognize in your workouts.
          </p>
          {error && (
            <Alert variant="destructive">
              <AlertTitle>Could not add exercise</AlertTitle>
              <AlertDescription id="exercise-error">{error}</AlertDescription>
            </Alert>
          )}
          <Button
            type="submit"
            disabled={saving || loading}
            className="mt-2 min-h-12 rounded-lg bg-primary px-4 py-3 font-semibold text-primary-foreground hover:bg-primary/90"
          >
            {saving ? "Saving…" : "Add exercise"}
          </Button>
        </form>
      </section>
    </div>
  );
}
