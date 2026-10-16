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
import { muscleGroupLabel, type Exercise } from "@/lib/exercises";
import {
  TemplateExerciseDialog,
  type TemplateExerciseDraft as DraftEntry,
} from "@/components/template-exercise-dialog";
import type { Template } from "@/lib/templates";
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
  const [description, setDescription] = useState(initial?.description ?? "");
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
  const [editing, setEditing] = useState<DraftEntry | null>(null);
  const [popupDirty, setPopupDirty] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [fields, setFields] = useState<Record<string, string>>({});
  const current = JSON.stringify({
    name,
    description,
    entries: entries.map(({ key, ...values }) => values),
  });
  const baseline = JSON.stringify({
    name: initial?.name ?? "",
    description: initial?.description ?? "",
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
    onDirtyChange?.(current !== baseline || popupDirty);
  }, [current, baseline, popupDirty, onDirtyChange]);
  useEffect(() => {
    onSavingChange?.(saving);
  }, [saving, onSavingChange]);
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
    if (saving || editing) return;
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
            description,
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
        setDescription("");
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
    <>
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
              aria-describedby={
                fields.name ? `error-${formID}-name` : undefined
              }
              placeholder="e.g. Push day"
            />
            {fieldError("name")}
          </div>
          <div>
            <label
              htmlFor={`${formID}-description`}
              className="mb-2 block text-sm font-medium"
            >
              Description (optional)
            </label>
            <textarea
              id={`${formID}-description`}
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              rows={3}
              className={`${inputClass} py-3`}
            />
          </div>
          <p
            id={`${formID}-reorder-help`}
            className="text-sm text-muted-foreground"
          >
            Drag an exercise card’s background to reorder, or hold briefly on
            touch screens. With a keyboard, focus the card, press Space, use
            arrow keys, then press Space to drop or Escape to cancel.
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
                  const exercise = exercises.find(
                    (exercise) => exercise.id === entry.exercise_id,
                  );
                  const label = exercise?.name ?? "Choose an exercise";
                  return (
                    <SortableExercise
                      key={entry.key}
                      id={entry.key}
                      label={`${label}, exercise ${index + 1}`}
                      disabled={saving || disabled}
                    >
                      <div className="min-w-0 rounded-2xl border border-border bg-card px-5 py-4 hover:bg-primary/5">
                        <div className="flex flex-wrap items-start justify-between gap-3">
                          <div className="min-w-0 flex-1">
                            <p className="break-words font-medium text-highlight">
                              {label}
                            </p>
                            {exercise?.description && (
                              <p className="mt-2 whitespace-pre-wrap break-words text-sm text-muted-foreground">
                                {exercise.description}
                              </p>
                            )}
                            <div className="mt-2 flex flex-wrap gap-2">
                              {exercise?.muscle_groups.map((group) => (
                                <span
                                  key={group}
                                  className="rounded-full bg-primary/10 px-2 py-1 text-xs text-highlight"
                                >
                                  {muscleGroupLabel(group)}
                                </span>
                              ))}
                            </div>
                            <p className="mt-3 break-words text-sm text-muted-foreground">
                              Exercise {index + 1} · {entry.set_count || "No"}{" "}
                              working sets
                              {(entry.rep_min || entry.rep_max) && (
                                <>
                                  {" "}
                                  · Reps: {entry.rep_min || "Not set"}–
                                  {entry.rep_max || "Not set"}
                                </>
                              )}
                              {entry.target_rir !== "" && (
                                <> · RIR {entry.target_rir}</>
                              )}
                            </p>
                            {[
                              "exercise_id",
                              "set_count",
                              "rep_range",
                              "target_rir",
                            ].map((field) => (
                              <div key={field}>
                                {fieldError(prefix + field)}
                              </div>
                            ))}
                          </div>
                          <div className="flex w-full flex-wrap gap-2 sm:w-auto">
                            <Button
                              type="button"
                              variant="outline"
                              aria-label={`Edit ${label}, exercise ${index + 1}`}
                              onClick={() => {
                                setPopupDirty(false);
                                setEditing(entry);
                              }}
                            >
                              Edit
                            </Button>
                            <Button
                              type="button"
                              variant="outline"
                              aria-label={`Delete ${label} from template, exercise ${index + 1}`}
                              onClick={() => {
                                setFields({});
                                setEntries((current) =>
                                  current.filter(
                                    (item) => item.key !== entry.key,
                                  ),
                                );
                              }}
                            >
                              Delete
                            </Button>
                          </div>
                        </div>
                      </div>
                    </SortableExercise>
                  );
                })}
              </div>
            </SortableContext>
          </DndContext>
          <p className="text-sm text-muted-foreground">
            Edit changes targets for this occurrence only. Delete removes it
            from the template, keeping the library exercise. Use Save template
            to save your changes.
          </p>
          {fieldError("exercises")}
          <Button
            type="button"
            variant="outline"
            disabled={disabled || exercises.length === 0}
            onClick={() => {
              setPopupDirty(false);
              setEditing({
                key: crypto.randomUUID(),
                exercise_id: "",
                set_count: "",
                rep_min: "",
                rep_max: "",
                target_rir: "",
              });
            }}
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
      {editing && (
        <TemplateExerciseDialog
          value={editing}
          exercises={exercises}
          fields={Object.fromEntries(
            Object.entries(fields)
              .filter(([key]) =>
                key.startsWith(
                  `exercises.${entries.findIndex((entry) => entry.key === editing.key)}.`,
                ),
              )
              .map(([key, value]) => [key.split(".").at(-1)!, value]),
          )}
          onDirtyChange={setPopupDirty}
          onClose={() => {
            setEditing(null);
            setPopupDirty(false);
          }}
          onApply={(saved) => {
            const index = entries.findIndex((entry) => entry.key === saved.key);
            setEntries((current) =>
              index < 0
                ? [...current, saved]
                : current.map((entry) =>
                    entry.key === saved.key ? saved : entry,
                  ),
            );
            setFields((current) =>
              Object.fromEntries(
                Object.entries(current).filter(
                  ([key]) => !key.startsWith(`exercises.${index}.`),
                ),
              ),
            );
            setEditing(null);
            setPopupDirty(false);
          }}
        />
      )}
    </>
  );
}
