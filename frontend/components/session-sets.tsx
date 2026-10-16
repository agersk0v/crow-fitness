"use client";
import { useEffect, useId, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import type { SavedSet } from "@/lib/sessions";

type Values = { weight: string; reps: string };
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
  const [sets, setSets] = useState(initial);
  const [entry, setEntry] = useState<Values>(() => ({
    weight: previousWeight ?? "",
    reps: "",
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
    discard: boolean;
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
        Object.keys(addFields).length > 0 ||
        entry.reps.trim() !== "",
    );
  }, [
    position,
    onBlocked,
    draft,
    busy,
    attempted,
    pendingDelete,
    entry.reps,
    addFields,
  ]);
  function resetAdd() {
    addID.current = null;
    setAttempted(false);
    setEntry((current) => ({ ...current, reps: "" }));
    setAddFields({});
    setError("");
  }
  async function save(editing: boolean) {
    if (inFlight.current || disabled || pendingDelete || (!editing && draft))
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
      setError(
        "Could not confirm the save. Retry using the same set, or discard the new set.",
      );
    } finally {
      inFlight.current = false;
      setBusy(false);
    }
  }
  async function remove(id: string, discard = false) {
    if (inFlight.current || disabled) return;
    inFlight.current = true;
    setBusy(true);
    setPendingDelete({ id, discard });
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
      if (discard) resetAdd();
    } catch {
      setError("Could not delete set. Please try again.");
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
          disabled={busy || disabled || !!draft || !!pendingDelete}
          onChange={(key, value) => {
            if (key === "weight") initialized.current = true;
            setEntry((current) => ({ ...current, [key]: value }));
          }}
        />
        <div className="flex flex-wrap gap-3">
          <Button
            type="submit"
            disabled={busy || disabled || !!draft || !!pendingDelete}
          >
            Add set
          </Button>
          {(entry.reps !== "" ||
            attempted ||
            Object.keys(addFields).length > 0) && (
            <Button
              type="button"
              variant="outline"
              disabled={busy || disabled || !!draft || !!pendingDelete}
              onClick={() => {
                if (attempted && addID.current)
                  void remove(addID.current, true);
                else resetAdd();
              }}
            >
              Discard set
            </Button>
          )}
        </div>
        {attempted && !busy && (
          <p className="text-xs text-muted-foreground">
            Retry Add set to confirm the save. Discard also removes it if the
            save reached the server.
          </p>
        )}
      </form>
      <h3 className="font-medium">Saved sets</h3>
      {sets.length === 0 ? (
        <p className="text-sm text-muted-foreground">No sets saved yet.</p>
      ) : (
        <ol className="space-y-2">
          {sets.map((set, index) => (
            <li
              key={set.id}
              className="flex flex-wrap items-center gap-3 rounded-lg border border-border p-3"
            >
              <span className="mr-auto text-sm">
                Set {index + 1}: {set.weight} kg × {set.reps} reps
              </span>
              <Button
                variant="outline"
                disabled={
                  disabled || busy || !!draft || !!pendingDelete || attempted
                }
                onClick={() => {
                  setError("");
                  setEditFields({});
                  setDraft({
                    id: set.id,
                    weight: set.weight,
                    reps: String(set.reps),
                    attempted: false,
                  });
                }}
              >
                Edit set {index + 1}
              </Button>
              <Button
                variant="outline"
                disabled={
                  disabled || busy || !!draft || !!pendingDelete || attempted
                }
                onClick={() => void remove(set.id)}
              >
                Delete set {index + 1}
              </Button>
            </li>
          ))}
        </ol>
      )}
      {draft && (
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
        </form>
      )}
      {busy && (
        <p role="status" className="text-sm text-muted-foreground">
          Saving changes…
        </p>
      )}
      {error && (
        <Alert variant="destructive">
          <AlertTitle>Set could not be saved or removed</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
          {pendingDelete && (
            <Button
              className="col-start-2 mt-3"
              disabled={busy || disabled}
              onClick={() =>
                void remove(pendingDelete.id, pendingDelete.discard)
              }
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
    <div className="grid grid-cols-2 gap-3">
      {(["weight", "reps"] as const).map((key) => (
        <div key={key}>
          <label htmlFor={`${id}-${key}`} className="mb-2 block text-sm">
            {key === "weight" ? "Weight (kg)" : "Reps"}
          </label>
          <input
            id={`${id}-${key}`}
            type="text"
            inputMode={key === "weight" ? "decimal" : "numeric"}
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
