"use client";
import { useEffect, useId, useRef, useState, type ReactNode } from "react";
import {
  DndContext,
  KeyboardSensor,
  closestCenter,
  useSensor,
  useSensors,
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
import type { SavedSet } from "@/lib/sessions";

type Values = { weight: string; reps: string; rir: string };
type EditDraft = Values & { id: string; attempted: boolean };
function validate(values: Values) {
  const fields: Record<string, string> = {};
  if (!/^(0|[1-9][0-9]*)(\.[0-9]{1,2})?$/.test(values.weight.trim()))
    fields.weight = "Enter nonnegative kilograms with at most two decimals.";
  if (
    !/^[0-9]+$/.test(values.reps) ||
    !Number.isSafeInteger(Number(values.reps)) ||
    Number(values.reps) <= 0
  )
    fields.reps = "Enter a positive whole-number rep count.";
  if (
    values.rir.trim() !== "" &&
    (!/^(0|[1-9][0-9]*)(\.[0-9]+)?$/.test(values.rir.trim()) ||
      !Number.isFinite(Number(values.rir)))
  )
    fields.rir = "Enter a nonnegative RIR, or leave it blank.";
  return fields;
}

export function SessionSets({
  sessionID,
  position,
  initial,
  disabled,
  onBlocked,
  previousWeight,
}: {
  sessionID: string;
  position: number;
  initial: SavedSet[];
  disabled: boolean;
  onBlocked: (position: number, blocked: boolean) => void;
  previousWeight?: string;
}) {
  const formID = useId();
  const sensors = useSensors(
    useSensor(CardMouseSensor, { activationConstraint: { distance: 6 } }),
    useSensor(CardTouchSensor, {
      activationConstraint: { delay: 250, tolerance: 5 },
    }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    }),
  );
  const [pendingOrder, setPendingOrder] = useState<SavedSet[] | null>(null);
  const [sets, setSets] = useState(initial);
  const [entry, setEntry] = useState<Values>(() => ({
    weight: previousWeight ?? "",
    reps: "",
    rir: "",
  }));
  const initialized = useRef(previousWeight !== undefined);
  const addID = useRef<string | null>(null);
  const inFlight = useRef(false);
  const [attempted, setAttempted] = useState(false);
  const [draft, setDraft] = useState<EditDraft | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [pendingDelete, setPendingDelete] = useState<{
    id: string;
  } | null>(null);
  const [addFields, setAddFields] = useState<Record<string, string>>({});
  const [editFields, setEditFields] = useState<Record<string, string>>({});

  useEffect(() => {
    if (!initialized.current && previousWeight !== undefined) {
      initialized.current = true;
      setEntry((current) => ({ ...current, weight: previousWeight }));
    }
  }, [previousWeight]);
  // A retained/default weight alone is not an unfinished set.
  useEffect(() => {
    onBlocked(
      position,
      !!draft ||
        busy ||
        attempted ||
        !!pendingDelete ||
        !!pendingOrder ||
        Object.keys(addFields).length > 0 ||
        entry.reps.trim() !== "" ||
        entry.rir.trim() !== "",
    );
  }, [
    position,
    onBlocked,
    draft,
    busy,
    attempted,
    pendingDelete,
    pendingOrder,
    entry.reps,
    entry.rir,
    addFields,
  ]);
  function resetAdd() {
    addID.current = null;
    setAttempted(false);
    setEntry((current) => ({ ...current, reps: "", rir: "" }));
    setAddFields({});
    setError("");
  }
  async function save(editing: boolean) {
    if (
      inFlight.current ||
      disabled ||
      pendingDelete ||
      pendingOrder ||
      (!editing && draft)
    )
      return;
    const values = editing ? draft : entry;
    if (!values) return;
    const fields = validate(values);
    if (editing) setEditFields(fields);
    else setAddFields(fields);
    setError("");
    if (Object.keys(fields).length) return;
    if (!editing) initialized.current = true;
    const id = editing ? draft!.id : (addID.current ??= crypto.randomUUID());
    inFlight.current = true;
    setBusy(true);
    if (editing)
      setDraft((current) => current && { ...current, attempted: true });
    else setAttempted(true);
    onBlocked(position, true);
    try {
      const response = await fetch(`/api/sessions/${sessionID}/sets/${id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          exercise_position: position,
          weight: values.weight.trim(),
          reps: Number(values.reps),
          rir: values.rir.trim() === "" ? null : Number(values.rir),
        }),
      });
      const body = await response.json();
      if (!response.ok) {
        if (editing) setEditFields(body.error?.fields ?? {});
        else setAddFields(body.error?.fields ?? {});
        setError(
          body.error?.message ?? "Could not save set. Please try again.",
        );
        return;
      }
      setSets((current) =>
        current.some((set) => set.id === body.id)
          ? current.map((set) => (set.id === body.id ? body : set))
          : [...current, body],
      );
      if (editing) {
        setDraft(null);
        setEditFields({});
      } else resetAdd();
    } catch {
      setError("Could not confirm the save. Retry using the same set.");
    } finally {
      inFlight.current = false;
      setBusy(false);
    }
  }
  async function remove(id: string) {
    if (inFlight.current || disabled) return;
    inFlight.current = true;
    setBusy(true);
    setPendingDelete({ id });
    setError("");
    onBlocked(position, true);
    try {
      const response = await fetch(`/api/sessions/${sessionID}/sets/${id}`, {
        method: "DELETE",
      });
      if (!response.ok) {
        const body = await response.json();
        setError(
          body.error?.message ?? "Could not delete set. Please try again.",
        );
        return;
      }
      setSets((current) => current.filter((set) => set.id !== id));
      setPendingDelete(null);
    } catch {
      setError("Could not delete set. Please try again.");
    } finally {
      inFlight.current = false;
      setBusy(false);
    }
  }
  async function reorder(next: SavedSet[]) {
    if (inFlight.current || disabled || draft || attempted || pendingDelete)
      return;
    inFlight.current = true;
    setBusy(true);
    setPendingOrder(next);
    setSets(next);
    setError("");
    onBlocked(position, true);
    try {
      const response = await fetch(`/api/sessions/${sessionID}/sets/order`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          exercise_position: position,
          set_ids: next.map((set) => set.id),
        }),
      });
      const body = await response.json();
      if (!response.ok) {
        setError(
          body.error?.message ??
            "Could not save set order. Retry the order or refresh to reload saved sets.",
        );
        return;
      }
      setSets(body.items);
      setPendingOrder(null);
    } catch {
      setError(
        "Could not confirm set order. Retry the same order or refresh to reload saved sets.",
      );
    } finally {
      inFlight.current = false;
      setBusy(false);
    }
  }

  return (
    <div className="mt-5 space-y-4">
      <form
        className="space-y-3"
        aria-label="Add a set"
        noValidate
        onSubmit={(event) => {
          event.preventDefault();
          void save(false);
        }}
      >
        <SetInputs
          id={`${formID}-add`}
          values={entry}
          fields={addFields}
          disabled={
            busy || disabled || !!draft || !!pendingDelete || !!pendingOrder
          }
          onChange={(key, value) => {
            if (key === "weight") initialized.current = true;
            const next = { ...entry, [key]: value };
            setEntry(next);
            setAddFields((current) => {
              const remaining = { ...current };
              delete remaining[key];
              return !attempted && !next.reps.trim() && !next.rir.trim()
                ? {}
                : remaining;
            });
            if (!attempted) setError("");
          }}
        />
        <div className="flex flex-wrap gap-3">
          <Button
            type="submit"
            disabled={
              busy || disabled || !!draft || !!pendingDelete || !!pendingOrder
            }
          >
            Add set
          </Button>
        </div>
        {attempted && !busy && (
          <p className="text-xs text-muted-foreground">
            Retry Add set to confirm the save.
          </p>
        )}
      </form>
      <h3 className="font-medium">Saved sets</h3>
      {sets.length === 0 ? (
        <p className="text-sm text-muted-foreground">No sets saved yet.</p>
      ) : (
        <DndContext
          sensors={sensors}
          collisionDetection={closestCenter}
          onDragEnd={({ active, over }) => {
            if (
              !over ||
              active.id === over.id ||
              busy ||
              disabled ||
              draft ||
              attempted ||
              pendingDelete ||
              pendingOrder
            )
              return;
            const from = sets.findIndex((set) => set.id === active.id),
              to = sets.findIndex((set) => set.id === over.id);
            if (from >= 0 && to >= 0) void reorder(arrayMove(sets, from, to));
          }}
        >
          <SortableContext
            items={sets.map((set) => set.id)}
            strategy={verticalListSortingStrategy}
          >
            <ol className="divide-y divide-border" aria-label="Saved sets">
              {sets.map((set, index) => (
                <li
                  key={set.id}
                  aria-label={`Set ${index + 1}`}
                  className="space-y-3 py-4"
                >
                  <SortableExercise
                    id={set.id}
                    label={`set ${index + 1}`}
                    kind="set"
                    disabled={
                      disabled ||
                      busy ||
                      !!draft ||
                      !!pendingDelete ||
                      attempted ||
                      !!pendingOrder
                    }
                  >
                    <div className="flex items-center justify-between gap-3">
                      <span className="text-lg font-semibold">
                        Set {index + 1}
                      </span>
                      <div className="flex gap-2">
                        <Button
                          variant="outline"
                          disabled={
                            disabled ||
                            busy ||
                            !!draft ||
                            !!pendingDelete ||
                            attempted ||
                            !!pendingOrder
                          }
                          onClick={() => {
                            setError("");
                            setEditFields({});
                            setDraft({
                              id: set.id,
                              weight: set.weight,
                              reps: String(set.reps),
                              rir: set.rir ?? "",
                              attempted: false,
                            });
                          }}
                        >
                          Edit
                        </Button>
                        <Button
                          variant="outline"
                          disabled={
                            disabled ||
                            busy ||
                            !!draft ||
                            !!pendingDelete ||
                            attempted ||
                            !!pendingOrder
                          }
                          onClick={() => void remove(set.id)}
                        >
                          Delete
                        </Button>
                      </div>
                    </div>
                    <dl className="mt-3 grid grid-cols-3 gap-3 text-sm">
                      <div>
                        <dt className="text-muted-foreground">Weight (kg)</dt>
                        <dd className="mt-1 tabular-nums">{set.weight}</dd>
                      </div>
                      <div>
                        <dt className="text-muted-foreground">Reps</dt>
                        <dd className="mt-1 tabular-nums">{set.reps}</dd>
                      </div>
                      <div>
                        <dt className="text-muted-foreground">RIR</dt>
                        <dd className="mt-1 tabular-nums">{set.rir ?? "—"}</dd>
                      </div>
                    </dl>
                  </SortableExercise>
                </li>
              ))}
            </ol>
          </SortableContext>
        </DndContext>
      )}
      {draft && (
        <SetEditDialog
          locked={busy || draft.attempted}
          onCancel={() => {
            setDraft(null);
            setEditFields({});
            setError("");
          }}
        >
          <form
            className="space-y-3 rounded-xl border border-border p-3"
            aria-label="Edit a set"
            noValidate
            onSubmit={(event) => {
              event.preventDefault();
              void save(true);
            }}
          >
            <p className="text-sm font-medium">Edit set</p>
            <SetInputs
              id={`${formID}-edit`}
              values={draft}
              fields={editFields}
              disabled={busy || disabled}
              onChange={(key, value) =>
                setDraft((current) => current && { ...current, [key]: value })
              }
            />
            <div className="flex flex-wrap gap-3">
              <Button type="submit" disabled={busy || disabled}>
                Save set
              </Button>
              <Button
                type="button"
                variant="outline"
                disabled={busy || disabled || draft.attempted}
                onClick={() => {
                  setDraft(null);
                  setError("");
                  setEditFields({});
                }}
              >
                Cancel edit
              </Button>
            </div>
            {draft.attempted && (
              <p className="text-xs text-muted-foreground">
                Retry Save to resolve this edit before cancelling it.
              </p>
            )}
            {error && (
              <p role="alert" className="text-sm text-destructive">
                {error}
              </p>
            )}
          </form>
        </SetEditDialog>
      )}
      {busy && (
        <p role="status" className="text-sm text-muted-foreground">
          Saving changes…
        </p>
      )}
      {error && !draft && (
        <Alert variant="destructive">
          <AlertTitle>Set could not be saved or removed</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
          {pendingOrder && (
            <Button
              className="col-start-2 mt-3"
              disabled={busy || disabled}
              onClick={() => void reorder(pendingOrder)}
            >
              Retry set order
            </Button>
          )}
          {pendingDelete && (
            <Button
              className="col-start-2 mt-3"
              disabled={busy || disabled}
              onClick={() => void remove(pendingDelete.id)}
            >
              Retry delete
            </Button>
          )}
        </Alert>
      )}
    </div>
  );
}

function SetInputs({
  id,
  values,
  fields,
  disabled,
  onChange,
}: {
  id: string;
  values: Values;
  fields: Record<string, string>;
  disabled: boolean;
  onChange: (key: keyof Values, value: string) => void;
}) {
  return (
    <div className="grid grid-cols-3 gap-3">
      {(["weight", "reps", "rir"] as const).map((key) => (
        <div key={key}>
          <label
            htmlFor={`${id}-${key}`}
            className="mb-2 block min-h-10 text-sm sm:min-h-0"
          >
            {key === "weight"
              ? "Weight (kg)"
              : key === "reps"
                ? "Reps"
                : "RIR (optional)"}
          </label>
          <input
            id={`${id}-${key}`}
            type="text"
            inputMode={key === "reps" ? "numeric" : "decimal"}
            value={values[key]}
            disabled={disabled}
            onChange={(event) => onChange(key, event.target.value)}
            aria-invalid={!!fields[key]}
            aria-describedby={fields[key] ? `${id}-${key}-error` : undefined}
            className="min-h-12 w-full min-w-0 rounded-lg border border-input bg-background px-3 text-base focus-visible:outline-2 focus-visible:outline-ring"
          />
          {fields[key] && (
            <p
              id={`${id}-${key}-error`}
              className="mt-1 text-sm text-destructive"
            >
              {fields[key]}
            </p>
          )}
        </div>
      ))}
    </div>
  );
}

function SetEditDialog({
  locked,
  onCancel,
  children,
}: {
  locked: boolean;
  onCancel: () => void;
  children: ReactNode;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const opener =
      document.activeElement instanceof HTMLElement
        ? document.activeElement
        : null;
    const dialog = ref.current;
    dialog?.showModal();
    return () => {
      dialog?.close();
      opener?.focus();
    };
  }, []);
  return (
    <dialog
      ref={ref}
      aria-label="Edit set"
      onCancel={(event) => {
        event.preventDefault();
        if (!locked) onCancel();
      }}
      className="m-auto max-h-[90dvh] w-[calc(100%-2rem)] max-w-lg overflow-y-auto rounded-2xl border border-border bg-card p-5 text-foreground shadow-xl backdrop:bg-black/60"
    >
      {children}
    </dialog>
  );
}
