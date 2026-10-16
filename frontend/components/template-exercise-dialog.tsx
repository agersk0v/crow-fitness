"use client";
import { useEffect, useId, useRef, useState, type FormEvent } from "react";
import { Button } from "@/components/ui/button";
import type { Exercise } from "@/lib/exercises";

export type ChoiceDraft = {
  exercise_id: string;
  set_count: string;
  rep_min: string;
  rep_max: string;
  target_rir: string;
};
export type TemplateExerciseDraft = ChoiceDraft & {
  key: string;
  alternatives: ChoiceDraft[];
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
        <h3 className="font-medium">Default exercise</h3>
        <ChoiceFields
          id={id}
          value={draft}
          exercises={exercises}
          fields={fields}
          excluded={draft.alternatives.map((c) => c.exercise_id)}
          onChange={(next) => setDraft((current) => ({ ...current, ...next }))}
        />
        <h3 className="font-medium">Alternatives</h3>
        {draft.alternatives.map((choice, index) => (
          <fieldset
            key={index}
            className="space-y-3 rounded-xl border border-border p-3"
          >
            <legend className="px-1 text-sm">Alternative {index + 1}</legend>
            <ChoiceFields
              id={`${id}-alt-${index}`}
              value={choice}
              exercises={exercises}
              fields={Object.fromEntries(
                Object.entries(fields)
                  .filter(([key]) => key.startsWith(`alternatives.${index}.`))
                  .map(([key, value]) => [
                    key.slice(`alternatives.${index}.`.length),
                    value,
                  ]),
              )}
              excluded={[
                draft.exercise_id,
                ...draft.alternatives
                  .filter((_, i) => i !== index)
                  .map((c) => c.exercise_id),
              ]}
              onChange={(next) =>
                setDraft((current) => ({
                  ...current,
                  alternatives: current.alternatives.map((c, i) =>
                    i === index ? next : c,
                  ),
                }))
              }
            />
            <div className="flex flex-wrap gap-2">
              <Button
                type="button"
                variant="outline"
                onClick={() =>
                  setDraft((current) => {
                    const { key, alternatives, ...defaultChoice } = current;
                    return {
                      key,
                      ...choice,
                      alternatives: alternatives.map((c, i) =>
                        i === index ? defaultChoice : c,
                      ),
                    };
                  })
                }
              >
                Make default
              </Button>
              <Button
                type="button"
                variant="outline"
                onClick={() =>
                  setDraft((current) => ({
                    ...current,
                    alternatives: current.alternatives.filter(
                      (_, i) => i !== index,
                    ),
                  }))
                }
              >
                Remove alternative
              </Button>
            </div>
          </fieldset>
        ))}
        <Button
          type="button"
          variant="outline"
          disabled={
            !draft.exercise_id ||
            draft.alternatives.length >= exercises.length - 1
          }
          onClick={() =>
            setDraft((current) => ({
              ...current,
              alternatives: [
                ...current.alternatives,
                {
                  exercise_id: "",
                  set_count: "3",
                  rep_min: "",
                  rep_max: "",
                  target_rir: "",
                },
              ],
            }))
          }
        >
          Add alternative
        </Button>
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

function ChoiceFields({
  id,
  value,
  exercises,
  fields,
  excluded,
  onChange,
}: {
  id: string;
  value: ChoiceDraft;
  exercises: Exercise[];
  fields: Record<string, string>;
  excluded: string[];
  onChange: (value: ChoiceDraft) => void;
}) {
  return (
    <div className="space-y-3">
      <label className="block text-sm font-medium" htmlFor={`${id}-exercise`}>
        Exercise
      </label>
      <select
        id={`${id}-exercise`}
        className={inputClass}
        value={value.exercise_id}
        onChange={(e) => onChange({ ...value, exercise_id: e.target.value })}
        aria-invalid={!!fields.exercise_id}
      >
        <option value="">Choose an exercise</option>
        {exercises
          .filter((e) => !excluded.includes(e.id))
          .map((e) => (
            <option key={e.id} value={e.id}>
              {e.name}
            </option>
          ))}
      </select>
      {fields.exercise_id && (
        <p role="alert" className="text-sm text-destructive">
          {fields.exercise_id}
        </p>
      )}
      <div className="grid gap-4 sm:grid-cols-2">
        {(
          [
            ["set_count", "Working-set count"],
            ["rep_min", "Minimum reps (optional)"],
            ["rep_max", "Maximum reps (optional)"],
            ["target_rir", "Target RIR (optional)"],
          ] as const
        ).map(([field, label]) => (
          <div key={field}>
            <label
              htmlFor={`${id}-${field}`}
              className="mb-2 block text-sm font-medium"
            >
              {label}
            </label>
            <input
              id={`${id}-${field}`}
              type="number"
              step={field === "target_rir" ? "any" : "1"}
              className={inputClass}
              value={value[field]}
              onChange={(e) => onChange({ ...value, [field]: e.target.value })}
              aria-invalid={
                !!fields[field.startsWith("rep_") ? "rep_range" : field]
              }
            />
          </div>
        ))}
      </div>
      {Object.entries(fields)
        .filter(([k]) => k !== "exercise_id")
        .map(([k, v]) => (
          <p role="alert" key={k} className="text-sm text-destructive">
            {v}
          </p>
        ))}
    </div>
  );
}
