"use client";

import { useState, type FormEvent } from "react";
import type { Exercise } from "@/lib/static-exercises";

export function ExerciseLibrary({
  initialExercises,
}: {
  initialExercises: Exercise[];
}) {
  const [exercises, setExercises] = useState(initialExercises);
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const [announcement, setAnnouncement] = useState("");

  function addExercise(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmedName = name.trim();
    if (!trimmedName) {
      setError("Enter an exercise name.");
      setAnnouncement("");
      return;
    }
    setExercises((current) => [
      ...current,
      { id: crypto.randomUUID(), name: trimmedName },
    ]);
    setName("");
    setError("");
    setAnnouncement(`${trimmedName} added.`);
  }

  return (
    <div className="grid gap-8 lg:grid-cols-[1fr_20rem]">
      <section aria-labelledby="exercise-list-heading" className="min-w-0">
        <h2 id="exercise-list-heading" className="mb-4 text-xl font-semibold">
          Exercise library
        </h2>
        {exercises.length === 0 ? (
          <p className="rounded-2xl border border-dashed border-white/15 p-6 text-slate-400">
            No exercises yet. Add your first exercise using the form.
          </p>
        ) : (
          <ul className="divide-y divide-white/10 rounded-2xl border border-white/10 bg-white/[0.03]">
            {exercises.map((exercise) => (
              <li
                key={exercise.id}
                className="break-words px-5 py-4 font-medium"
              >
                {exercise.name}
              </li>
            ))}
          </ul>
        )}
      </section>
      <section
        aria-labelledby="add-exercise-heading"
        className="self-start rounded-2xl border border-white/10 bg-white/[0.03] p-5"
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
            onChange={(event) => setName(event.target.value)}
            aria-invalid={Boolean(error)}
            aria-describedby={
              error ? "exercise-help exercise-error" : "exercise-help"
            }
            className="min-h-12 w-full rounded-lg border border-white/20 bg-slate-950 px-3 text-base focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-lime-300"
            placeholder="e.g. Overhead press"
          />
          <p id="exercise-help" className="text-sm text-slate-400">
            Choose a name you will recognize in your workouts.
          </p>
          {error && (
            <p
              id="exercise-error"
              role="alert"
              className="text-sm text-red-300"
            >
              {error}
            </p>
          )}
          <button
            type="submit"
            className="mt-2 min-h-12 rounded-lg bg-lime-300 px-4 py-3 font-semibold text-slate-950 hover:bg-lime-200"
          >
            Add exercise
          </button>
        </form>
        <p role="status" className="mt-3 text-sm text-lime-300">
          {announcement}
        </p>
      </section>
    </div>
  );
}
