"use client";
import { useEffect, useId, useState, type FormEvent } from "react";
import { toast } from "sonner";
import {
  DndContext,
  KeyboardSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
} from "@dnd-kit/core";
import {
  SortableContext,
  arrayMove,
  sortableKeyboardCoordinates,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";
import { CardMouseSensor, CardTouchSensor } from "@/lib/template-drag-sensors";
import { SortableExercise } from "@/components/sortable-exercise";
import { Button } from "@/components/ui/button";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import type { Exercise } from "@/lib/exercises";
import type { Template } from "@/lib/templates";
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
export function TemplateForm({
  exercises,
  initial,
  disabled = false,
  onSaved,
  onCancel,
  onDirtyChange,
  onSavingChange,
}: {
  exercises: Exercise[];
  initial?: Template;
  disabled?: boolean;
  onSaved: (template: Template) => void;
  onCancel?: () => void;
  onDirtyChange?: (dirty: boolean) => void;
  onSavingChange?: (saving: boolean) => void;
}) {
  const formID = useId();
  const [name, setName] = useState(initial?.name ?? "");
  const [entries, setEntries] = useState<DraftEntry[]>(
    () =>
      initial?.exercises.map((entry) => ({
        key: crypto.randomUUID(),
        exercise_id: entry.exercise_id,
        set_count: String(entry.set_count),
        rep_min: entry.rep_min === null ? "" : String(entry.rep_min),
        rep_max: entry.rep_max === null ? "" : String(entry.rep_max),
        target_rir: entry.target_rir === null ? "" : String(entry.target_rir),
      })) ?? [],
  );
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [fields, setFields] = useState<Record<string, string>>({});
  const current = JSON.stringify({
    name,
    entries: entries.map(({ key, ...values }) => values),
  });
  const baseline = JSON.stringify({
    name: initial?.name ?? "",
    entries:
      initial?.exercises.map((entry) => ({
        exercise_id: entry.exercise_id,
        set_count: String(entry.set_count),
        rep_min: entry.rep_min === null ? "" : String(entry.rep_min),
        rep_max: entry.rep_max === null ? "" : String(entry.rep_max),
        target_rir: entry.target_rir === null ? "" : String(entry.target_rir),
      })) ?? [],
  });
  useEffect(() => {
    onDirtyChange?.(current !== baseline);
  }, [current, baseline, onDirtyChange]);
  useEffect(() => {
    onSavingChange?.(saving);
  }, [saving, onSavingChange]);
  function update(key: string, field: keyof DraftEntry, value: string) {
    setEntries((current) =>
      current.map((entry) =>
        entry.key === key ? { ...entry, [field]: value } : entry,
      ),
    );
  }
  const sensors = useSensors(
    useSensor(CardMouseSensor, { activationConstraint: { distance: 8 } }),
    useSensor(CardTouchSensor, {
      activationConstraint: { delay: 250, tolerance: 5 },
    }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    }),
  );
  function describe(key: string | number) {
    const index = entries.findIndex((entry) => entry.key === key);
    const entry = entries[index];
    return (
      exercises.find((exercise) => exercise.id === entry?.exercise_id)?.name ??
      `Exercise ${index + 1}`
    );
  }
  function reorder({ active, over }: DragEndEvent) {
    if (saving || disabled || !over || active.id === over.id) return;
    setFields({});
    setEntries((current) => {
      const from = current.findIndex((entry) => entry.key === active.id);
      const to = current.findIndex((entry) => entry.key === over.id);
      return from < 0 || to < 0 ? current : arrayMove(current, from, to);
    });
  }
  function fieldError(field: string) {
    return fields[field] ? (
      <p id={`error-${formID}-${field}`} className="text-sm text-destructive">
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
      const response = await fetch(
        initial ? `/api/templates/${initial.id}` : "/api/templates",
        {
          method: initial ? "PUT" : "POST",
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
        },
      );
      const body = await response.json();
      if (!response.ok) {
        setError(
          body.error?.message ?? "Could not save template. Please try again.",
        );
        setFields(body.error?.fields ?? {});
        return;
      }
      if (!initial) {
        setName("");
        setEntries([]);
      }
      onSaved(body);
      toast.success(`${body.name} saved.`);
    } catch {
      setError("Could not save template. Please try again.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <form onSubmit={save} noValidate className="mt-5 space-y-5">
      <fieldset disabled={saving || disabled} className="min-w-0 space-y-5">
        <div>
          <label
            htmlFor={`${formID}-name`}
            className="mb-2 block text-sm font-medium"
          >
            Template name
          </label>
          <input
            id={`${formID}-name`}
            value={name}
            onChange={(event) => setName(event.target.value)}
            className={inputClass}
            aria-invalid={Boolean(fields.name)}
            aria-describedby={fields.name ? `error-${formID}-name` : undefined}
            placeholder="e.g. Push day"
          />
          {fieldError("name")}
        </div>
        <p
          id={`${formID}-reorder-help`}
          className="text-sm text-muted-foreground"
        >
          Drag an exercise card’s background to reorder, or hold briefly on
          touch screens. With a keyboard, focus the card, press Space, use arrow
          keys, then press Space to drop or Escape to cancel.
        </p>
        <DndContext
          sensors={sensors}
          collisionDetection={closestCenter}
          onDragEnd={reorder}
          accessibility={{
            announcements: {
              onDragStart: ({ active }) =>
                `Picked up ${describe(active.id)}. Use arrow keys to move, Space to drop, or Escape to cancel.`,
              onDragOver: ({ active, over }) =>
                over
                  ? `${describe(active.id)}, position ${entries.findIndex((entry) => entry.key === over.id) + 1} of ${entries.length}.`
                  : undefined,
              onDragEnd: ({ active, over }) =>
                over
                  ? `${describe(active.id)} dropped at position ${entries.findIndex((entry) => entry.key === over.id) + 1}.`
                  : "Reordering cancelled.",
              onDragCancel: () => "Reordering cancelled.",
            },
          }}
        >
          <SortableContext
            items={entries.map((entry) => entry.key)}
            strategy={verticalListSortingStrategy}
          >
            <div
              className="space-y-5"
              aria-describedby={`${formID}-reorder-help`}
            >
              {entries.map((entry, index) => {
                const prefix = `exercises.${index}.`;
                const label =
                  exercises.find(
                    (exercise) => exercise.id === entry.exercise_id,
                  )?.name ?? `Exercise ${index + 1}`;
                return (
                  <SortableExercise
                    key={entry.key}
                    id={entry.key}
                    label={`${label}, exercise ${index + 1}`}
                    disabled={saving || disabled}
                  >
                    <fieldset className="min-w-0 rounded-xl border border-border p-4">
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
                            ? `error-${formID}-${prefix}exercise_id`
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
                                  fields[errorKey]
                                    ? `error-${formID}-${errorKey}`
                                    : undefined
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
                  </SortableExercise>
                );
              })}
            </div>
          </SortableContext>
        </DndContext>
        {fieldError("exercises")}
        <Button
          type="button"
          variant="outline"
          disabled={disabled || exercises.length === 0}
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
        disabled={saving || disabled || exercises.length === 0}
        className="min-h-12 w-full sm:w-auto"
      >
        {saving ? "Saving…" : "Save template"}
      </Button>
      {onCancel && (
        <Button
          type="button"
          variant="outline"
          disabled={saving || disabled}
          onClick={onCancel}
        >
          Cancel
        </Button>
      )}
    </form>
  );
}
