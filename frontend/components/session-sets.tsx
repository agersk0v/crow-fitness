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
import {
  techniqueLabel,
  partialRepsLabel,
  type Technique,
  type SavedSet,
  type Session,
} from "@/lib/sessions";

import { ChevronDown } from "lucide-react";
import type { TemplateExercise } from "@/lib/templates";

type Values = {
  partial_reps: string;
  weight: string;
  reps: string;
  rir: string;
  technique: Technique;
};
type EditDraft = Values & { id: string; attempted: boolean };
function validate(values: Values) {
  const fields: Record<string, string> = {};
  if (
    values.partial_reps.trim() !== "" &&
    (!/^[0-9]+$/.test(values.partial_reps.trim()) ||
      !Number.isSafeInteger(Number(values.partial_reps)))
  )
    fields.partial_reps =
      "Enter a nonnegative whole-number partial-rep count, or leave it blank.";
  if (!["good", "decent", "bad"].includes(values.technique))
    fields.technique = "Choose Good, Decent, or Bad technique.";
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
  plannedCount,
  exercise,
  onSelected,
  children,
}: {
  children: ReactNode;
  exercise: TemplateExercise;
  onSelected: (session: Session, position: number) => void;
  sessionID: string;
  position: number;
  initial: SavedSet[];
  disabled: boolean;
  onBlocked: (position: number, blocked: boolean) => void;
  previousWeight?: string;
  plannedCount: number;
}) {
  const formID = useId();
  const [switchTo, setSwitchTo] = useState<string | null>(null);
  const [switchPending, setSwitchPending] = useState<string | null>(null);
  const [switchBusy, setSwitchBusy] = useState(false);
  const [switchError, setSwitchError] = useState("");
  const revision = exercise.selection_revision ?? 0;
  const externallyDisabled = disabled;
  disabled = disabled || switchBusy || !!switchPending || !!switchTo;
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
  const atLimit = sets.length >= plannedCount;
  const [entry, setEntry] = useState<Values>(() => ({
    weight: previousWeight ?? "",
    reps: "",
    rir: "",
    partial_reps: "",
    technique: "decent",
  }));
  const initialized = useRef(previousWeight !== undefined);
  const addID = useRef<string | null>(null);
  const inFlight = useRef(false);
  const [attempted, setAttempted] = useState(false);
  const [staleCount, setStaleCount] = useState("");
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
      switchBusy ||
        !!switchPending ||
        !!switchTo ||
        !!draft ||
        busy ||
        attempted ||
        !!staleCount ||
        !!pendingDelete ||
        !!pendingOrder ||
        Object.keys(addFields).length > 0 ||
        entry.reps.trim() !== "" ||
        entry.rir.trim() !== "" ||
        entry.partial_reps.trim() !== "",
    );
  }, [
    position,
    onBlocked,
    draft,
    busy,
    attempted,
    staleCount,
    pendingDelete,
    pendingOrder,
    entry.reps,
    entry.rir,
    entry.partial_reps,
    addFields,
    switchBusy,
    switchPending,
    switchTo,
  ]);
  function resetAdd() {
    addID.current = null;
    setAttempted(false);
    setEntry((current) => ({
      ...current,
      reps: "",
      rir: "",
      partial_reps: "",
      technique: "decent",
    }));
    setAddFields({});
    setError("");
  }
  async function save(editing: boolean) {
    if (
      inFlight.current ||
      disabled ||
      pendingDelete ||
      pendingOrder ||
      (!editing && draft) ||
      (!editing && (staleCount || (atLimit && !attempted)))
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
          selection_revision: revision,
          technique: values.technique,
          partial_reps:
            values.partial_reps.trim() === ""
              ? null
              : Number(values.partial_reps),
          weight: values.weight.trim(),
          reps: Number(values.reps),
          rir: values.rir.trim() === "" ? null : Number(values.rir),
        }),
      });
      const body = await response.json();
      if (!response.ok) {
        if (body.error?.code === "selection_conflict") {
          setStaleCount(
            "This slot’s exercise changed in another tab. Reload this slot to use its saved selection; your unsaved inputs for this slot will be discarded.",
          );
          setAttempted(false);
          if (editing) {
            setDraft((current) => current && { ...current, attempted: false });
            setError(
              "The exercise selection changed. Cancel this edit, then reload this slot to use the saved selection.",
            );
          }
          return;
        }
        if (
          !editing &&
          response.status === 400 &&
          body.error?.code === "validation_error" &&
          typeof body.error?.fields?.set_count === "string"
        ) {
          // Go checks this cap only for a new UUID: this addition was rejected,
          // unlike a lost response that must still retry the same UUID.
          addID.current = null;
          setAttempted(false);
          setAddFields({});
          setStaleCount(body.error.fields.set_count);
          return;
        }
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
  async function reloadSavedSets() {
    if (inFlight.current || disabled || !staleCount) return;
    inFlight.current = true;
    setBusy(true);
    setError("");
    try {
      const response = await fetch(`/api/sessions/${sessionID}`, {
        cache: "no-store",
      });
      if (!response.ok) throw new Error();
      const saved: Session = await response.json();
      if (
        saved.exercises.find((e) => e.position === position)
          ?.selection_revision !== revision
      ) {
        onSelected(saved, position);
        return;
      }
      setSets(saved.sets.filter((set) => set.exercise_position === position));
      // Only this exercise's rejected addition is cleared after an explicit,
      // successful reload. Other mounted exercise drafts stay untouched.
      resetAdd();
      setStaleCount("");
    } catch {
      setError(
        "Could not reload saved sets. Your inputs have been kept. Try again.",
      );
    } finally {
      inFlight.current = false;
      setBusy(false);
    }
  }
  async function remove(id: string) {
    if (inFlight.current || disabled || staleCount) return;
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
    if (
      inFlight.current ||
      disabled ||
      staleCount ||
      draft ||
      attempted ||
      pendingDelete
    )
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
          selection_revision: revision,
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

  const switchingBlocked =
    !!draft ||
    busy ||
    attempted ||
    !!staleCount ||
    !!pendingDelete ||
    !!pendingOrder;
  const hasDraft =
    entry.weight !== (previousWeight ?? "") ||
    !!entry.reps.trim() ||
    !!entry.rir.trim() ||
    !!entry.partial_reps.trim() ||
    entry.technique !== "decent" ||
    Object.keys(addFields).length > 0;
  async function selectExercise(target: string) {
    if (inFlight.current || externallyDisabled) return;
    inFlight.current = true;
    setSwitchBusy(true);
    setSwitchTo(null);
    setSwitchError("");
    onBlocked(position, true);
    try {
      const response = await fetch(`/api/sessions/${sessionID}/selection`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          exercise_position: position,
          exercise_id: target,
          selection_revision: revision,
        }),
      });
      if (response.ok) {
        const saved: Session = await response.json();
        onSelected(saved, position);
        return;
      }
      const body = await response.json();
      if (body.error?.code === "selection_conflict") {
        // The revision has advanced, so retrying the original CAS can no longer
        // switch this slot. Reconcile only this slot; other drafts stay mounted.
        setSwitchPending(target);
        const current = await fetch(`/api/sessions/${sessionID}`, {
          cache: "no-store",
        });
        if (!current.ok) throw new Error();
        const saved: Session = await current.json();
        if (
          saved.exercises.find((e) => e.position === position)
            ?.selection_revision === revision
        )
          throw new Error();
        onSelected(saved, position);
        return;
      }
      if (response.status >= 500 || switchPending) {
        setSwitchPending(target);
      }
      if (body.error?.code === "exercise_has_sets") {
        // This revision cannot switch while sets survive. Require a fresh
        // snapshot before allowing writes, even after an uncertain attempt.
        setSwitchPending(null);
        setStaleCount(
          "Saved sets were added in another tab. Reload this slot before switching; its unsaved inputs will be discarded.",
        );
      }
      setSwitchError(
        body.error?.message ??
          "Could not change exercise. Your selection and inputs have been kept.",
      );
    } catch {
      setSwitchPending(target);
      setSwitchError(
        "Could not confirm the selection. Resolve it before adding sets. Your inputs have been kept until the saved selection is known.",
      );
    } finally {
      inFlight.current = false;
      setSwitchBusy(false);
    }
  }
  return (
    <div className="space-y-4">
      <h2
        aria-label={exercise.name}
        className="min-w-0 break-words text-xl font-semibold"
      >
        {(exercise.choices?.length ?? 0) > 1 ? (
          <span className="group relative block">
            <select
              id={`${formID}-choice`}
              aria-label="Exercise choice"
              value={exercise.exercise_id}
              disabled={disabled || switchingBlocked || sets.length > 0}
              className="min-h-11 w-full min-w-0 cursor-pointer appearance-none rounded-md border-0 bg-transparent py-1 pr-7 text-xl font-semibold text-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-default"
              onChange={(e) => {
                const target = e.target.value;
                if (target === exercise.exercise_id) return;
                if (hasDraft) setSwitchTo(target);
                else void selectExercise(target);
              }}
            >
              {exercise.choices!.map((c) => (
                <option key={c.exercise_id} value={c.exercise_id}>
                  {c.name}
                </option>
              ))}
            </select>
            <ChevronDown
              aria-hidden="true"
              className="pointer-events-none absolute right-0 top-1/2 size-4 -translate-y-1/2 text-muted-foreground opacity-50 group-hover:opacity-100"
            />
          </span>
        ) : (
          exercise.name
        )}
      </h2>
      {children}
      {(exercise.choices?.length ?? 0) > 1 && (
        <div className="space-y-2">
          {sets.length > 0 && (
            <p className="text-sm text-muted-foreground">
              Delete all saved sets in this slot to change exercise.
            </p>
          )}
          {switchingBlocked && sets.length === 0 && (
            <p className="text-sm text-muted-foreground">
              Resolve this slot’s unfinished changes before switching.
            </p>
          )}
          {switchError && (
            <p role="alert" className="text-sm text-destructive">
              {switchError}
            </p>
          )}
          {switchBusy && <p role="status">Changing exercise…</p>}
          {switchPending && (
            <Button
              disabled={switchBusy || externallyDisabled}
              variant="outline"
              onClick={() => void selectExercise(switchPending)}
            >
              Resolve exercise selection
            </Button>
          )}
        </div>
      )}
      {switchTo && (
        <SwitchDialog
          onCancel={() => setSwitchTo(null)}
          onConfirm={() => void selectExercise(switchTo)}
        />
      )}

      {staleCount && (
        <Alert variant="destructive">
          <AlertTitle>Saved sets changed</AlertTitle>
          <AlertDescription>
            <p>{staleCount}</p>
            <p className="mt-2">
              Reload saved sets updates this exercise and clears its unsaved
              reps, RIR, partial reps, and technique selection (reset to
              Decent). Your weight and drafts for other exercises are kept.
            </p>
            {error && <p className="mt-2">{error}</p>}
          </AlertDescription>
          <Button
            className="col-start-2 mt-3"
            disabled={busy || disabled}
            onClick={() => void reloadSavedSets()}
          >
            {busy ? "Reloading saved sets…" : "Reload saved sets"}
          </Button>
        </Alert>
      )}
      {atLimit && !attempted ? (
        <p role="status" className="text-sm font-medium text-highlight">
          Exercise complete ({sets.length}/{plannedCount} sets saved).
        </p>
      ) : (
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
              busy ||
              disabled ||
              !!draft ||
              !!pendingDelete ||
              !!pendingOrder ||
              (atLimit && !attempted)
            }
            onChange={(key, value) => {
              if (key === "weight") initialized.current = true;
              const next = { ...entry, [key]: value };
              setEntry(next);
              setAddFields((current) => {
                const remaining = { ...current };
                delete remaining[key];
                return !attempted &&
                  !next.reps.trim() &&
                  !next.rir.trim() &&
                  !next.partial_reps.trim()
                  ? {}
                  : remaining;
              });
              if (!attempted && !staleCount) setError("");
            }}
          />
          <div className="flex flex-wrap gap-3">
            <Button
              type="submit"
              disabled={
                busy ||
                disabled ||
                !!draft ||
                !!pendingDelete ||
                !!pendingOrder ||
                !!staleCount ||
                (atLimit && !attempted)
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
      )}
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
              staleCount ||
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
                      !!staleCount ||
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
                            !!staleCount ||
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
                              partial_reps:
                                set.partial_reps == null
                                  ? ""
                                  : String(set.partial_reps),
                              technique: set.technique,
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
                            !!staleCount ||
                            !!pendingOrder
                          }
                          onClick={() => void remove(set.id)}
                        >
                          Delete
                        </Button>
                      </div>
                    </div>
                    <dl className="mt-3 grid grid-cols-2 gap-3 text-sm sm:grid-cols-4">
                      <div>
                        <dt className="text-muted-foreground">Weight (kg)</dt>
                        <dd className="mt-1 tabular-nums">{set.weight}</dd>
                      </div>
                      <div>
                        <dt className="text-muted-foreground">Reps</dt>
                        <dd className="mt-1 tabular-nums">
                          {set.reps}
                          {!!set.partial_reps && (
                            <span className="mt-1 block text-xs text-muted-foreground">
                              {partialRepsLabel(set.partial_reps)}
                            </span>
                          )}
                        </dd>
                      </div>
                      <div>
                        <dt className="text-muted-foreground">RIR</dt>
                        <dd className="mt-1 tabular-nums">{set.rir ?? "—"}</dd>
                      </div>
                      <div>
                        <dt className="text-muted-foreground">Technique</dt>
                        <dd className="mt-1">
                          {techniqueLabel(set.technique)}
                        </dd>
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
      {error && !draft && !staleCount && (
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
      <div className="col-span-2">
        <label
          htmlFor={`${id}-technique`}
          className="mb-2 block min-h-10 text-sm sm:min-h-0"
        >
          Technique
        </label>
        <select
          id={`${id}-technique`}
          value={values.technique}
          disabled={disabled}
          onChange={(event) => onChange("technique", event.target.value)}
          aria-invalid={!!fields.technique}
          aria-describedby={
            fields.technique ? `${id}-technique-error` : undefined
          }
          className="min-h-12 w-full rounded-lg border border-input bg-background px-3 text-base focus-visible:outline-2 focus-visible:outline-ring"
        >
          <option value="good">Good</option>
          <option value="decent">Decent</option>
          <option value="bad">Bad</option>
        </select>
        {fields.technique && (
          <p
            id={`${id}-technique-error`}
            className="mt-1 text-sm text-destructive"
          >
            {fields.technique}
          </p>
        )}
      </div>
      <div>
        <label
          htmlFor={`${id}-partial_reps`}
          className="mb-2 block min-h-10 text-sm sm:min-h-0"
        >
          Partial reps (optional)
        </label>
        <input
          id={`${id}-partial_reps`}
          type="text"
          inputMode="numeric"
          value={values.partial_reps}
          disabled={disabled}
          onChange={(event) => onChange("partial_reps", event.target.value)}
          aria-invalid={!!fields.partial_reps}
          aria-describedby={
            fields.partial_reps ? `${id}-partial_reps-error` : undefined
          }
          className="min-h-12 w-full min-w-0 rounded-lg border border-input bg-background px-3 text-base focus-visible:outline-2 focus-visible:outline-ring"
        />
        {fields.partial_reps && (
          <p
            id={`${id}-partial_reps-error`}
            className="mt-1 text-sm text-destructive"
          >
            {fields.partial_reps}
          </p>
        )}
      </div>
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

function SwitchDialog({
  onCancel,
  onConfirm,
}: {
  onCancel: () => void;
  onConfirm: () => void;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const opener = document.activeElement;
    ref.current?.showModal();
    return () => {
      ref.current?.close();
      if (opener instanceof HTMLElement) opener.focus();
    };
  }, []);
  return (
    <dialog
      ref={ref}
      aria-label="Discard inputs and change exercise?"
      onCancel={(e) => {
        e.preventDefault();
        onCancel();
      }}
      className="m-auto w-[calc(100%-2rem)] max-w-lg rounded-2xl border border-border bg-card p-5 text-foreground backdrop:bg-black/60"
    >
      <h2 className="text-xl font-semibold">
        Discard inputs and change exercise?
      </h2>
      <p className="my-4 text-sm text-muted-foreground">
        This clears this slot’s unsaved weight, reps, RIR, partial reps, and
        technique. The chosen exercise starts with its previous weight and
        Decent technique.
      </p>
      <div className="flex flex-wrap gap-3">
        <Button autoFocus variant="outline" onClick={onCancel}>
          Keep inputs
        </Button>
        <Button onClick={onConfirm}>Discard inputs and switch</Button>
      </div>
    </dialog>
  );
}
