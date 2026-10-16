"use client";
import { useEffect, useId, useRef, useState, type FormEvent } from "react";
import { Button } from "@/components/ui/button";
import type { Exercise } from "@/lib/exercises";

export type TemplateExerciseDraft = {
  key: string;
  exercise_id: string;
  set_count: string;
  rep_min: string;
  rep_max: string;
  target_rir: string;
};
const inputClass =
  "min-h-12 w-full rounded-lg border border-input bg-background px-3 text-base focus-visible:outline-2 focus-visible:outline-ring";
export function TemplateExerciseDialog({
  value,
  exercises,
  fields,
  onApply,
  onClose,
  onDirtyChange,
}: {
  value: TemplateExerciseDraft;
  exercises: Exercise[];
  fields: Record<string, string>;
  onApply: (entry: TemplateExerciseDraft) => void;
  onClose: () => void;
  onDirtyChange: (dirty: boolean) => void;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const id = useId();
  const [draft, setDraft] = useState(value);
  const choosing = value.exercise_id === "";
  const exercise = exercises.find(
    (exercise) => exercise.id === draft.exercise_id,
  );
  useEffect(() => {
    const modal = dialog.current;
    const opener = document.activeElement;
    modal?.showModal();
    return () => {
      modal?.close();
      if (opener instanceof HTMLElement && opener.isConnected) opener.focus();
    };
  }, []);
  const dirty = JSON.stringify(draft) !== JSON.stringify(value);
  useEffect(() => {
    onDirtyChange(dirty);
  }, [dirty, onDirtyChange]);
  function error(field: string) {
    return fields[field] ? (
      <p id={`${id}-${field}-error`} className="text-sm text-destructive">
        {fields[field]}
      </p>
    ) : null;
  }
  return (
    <dialog
      ref={dialog}
      aria-labelledby={`${id}-title`}
      aria-describedby={`${id}-description`}
      onCancel={(event) => {
        event.preventDefault();
        onClose();
      }}
      className="m-auto max-h-[90dvh] w-[calc(100%-2rem)] max-w-lg overflow-y-auto rounded-2xl border border-border bg-card p-5 text-foreground shadow-xl backdrop:bg-black/60"
    >
      <h2 id={`${id}-title`} className="break-words text-xl font-semibold">
        {choosing ? "Add exercise to template" : "Edit exercise targets"}
      </h2>
      <p
        id={`${id}-description`}
        className="mt-3 break-words text-sm text-muted-foreground"
      >
        {exercise?.name && <>{exercise.name}. </>}These targets apply to this
        occurrence only. Apply changes updates your draft; Save template commits
        it. Exercise names, tags, and descriptions are managed on the Exercises
        page.
      </p>
      <form
        className="mt-5 space-y-4"
        noValidate
        onSubmit={(event: FormEvent<HTMLFormElement>) => {
          event.preventDefault();
          onApply(draft);
        }}
      >
        {choosing && (
          <div>
            <label
              htmlFor={`${id}-exercise`}
              className="mb-2 block text-sm font-medium"
            >
              Exercise
            </label>
            <select
              autoFocus
              id={`${id}-exercise`}
              value={draft.exercise_id}
              className={inputClass}
              aria-invalid={Boolean(fields.exercise_id)}
              aria-describedby={
                fields.exercise_id ? `${id}-exercise_id-error` : undefined
              }
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  exercise_id: event.target.value,
                }))
              }
            >
              <option value="">Choose an exercise</option>
              {exercises.map((exercise) => (
                <option key={exercise.id} value={exercise.id}>
                  {exercise.name}
                </option>
              ))}
            </select>
            {error("exercise_id")}
          </div>
        )}
        <div className="grid gap-4 sm:grid-cols-2">
          {(
            [
              ["set_count", "Working-set count"],
              ["rep_min", "Minimum reps (optional)"],
              ["rep_max", "Maximum reps (optional)"],
              ["target_rir", "Target RIR (optional)"],
            ] as const
          ).map(([field, label]) => {
            const errorKey = field.startsWith("rep_") ? "rep_range" : field;
            return (
              <div key={field}>
                <label
                  htmlFor={`${id}-${field}`}
                  className="mb-2 block text-sm font-medium"
                >
                  {label}
                </label>
                <input
                  autoFocus={!choosing && field === "set_count"}
                  id={`${id}-${field}`}
                  type="number"
                  step={field === "target_rir" ? "any" : "1"}
                  value={draft[field]}
                  className={inputClass}
                  aria-invalid={Boolean(fields[errorKey])}
                  aria-describedby={
                    fields[errorKey] ? `${id}-${errorKey}-error` : undefined
                  }
                  onChange={(event) =>
                    setDraft((current) => ({
                      ...current,
                      [field]: event.target.value,
                    }))
                  }
                />
                {!field.startsWith("rep_") && error(errorKey)}
              </div>
            );
          })}
        </div>
        {error("rep_range")}
        <p className="text-sm text-muted-foreground">
          Targets are validated when you save the template.
        </p>
        <div className="flex flex-wrap gap-3">
          <Button type="submit">Apply changes</Button>
          <Button type="button" variant="outline" onClick={onClose}>
            Cancel
          </Button>
        </div>
      </form>
    </dialog>
  );
}
