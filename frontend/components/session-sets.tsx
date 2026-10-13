"use client";
import { useId, useState } from "react";
import { Button } from "@/components/ui/button";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import type { SavedSet } from "@/lib/sessions";

type Draft = {
  id: string;
  weight: string;
  reps: string;
  saved: boolean;
  attempted: boolean;
};
export function SessionSets({
  sessionID,
  position,
  initial,
  disabled,
  onBlocked,
}: {
  sessionID: string;
  position: number;
  initial: SavedSet[];
  disabled: boolean;
  onBlocked: (position: number, blocked: boolean) => void;
}) {
  const formID = useId();
  const [sets, setSets] = useState(initial);
  const [draft, setDraft] = useState<Draft | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [pendingDelete, setPendingDelete] = useState<string | null>(null);
  const [fields, setFields] = useState<Record<string, string>>({});
  function edit(next: Draft) {
    setError("");
    setFields({});
    setDraft(next);
    onBlocked(position, true);
  }
  function finish() {
    setPendingDelete(null);
    setDraft(null);
    setError("");
    setFields({});
    onBlocked(position, false);
  }
  async function save() {
    if (!draft || busy || disabled) return;
    const errors: Record<string, string> = {};
    const weight = draft.weight.trim(),
      reps = Number(draft.reps);
    if (!/^(0|[1-9][0-9]*)(\.[0-9]{1,2})?$/.test(weight))
      errors.weight = "Enter nonnegative kilograms with at most two decimals.";
    if (
      !/^[0-9]+$/.test(draft.reps) ||
      !Number.isSafeInteger(reps) ||
      reps <= 0
    )
      errors.reps = "Enter a positive whole-number rep count.";
    setFields(errors);
    setError("");
    if (Object.keys(errors).length) return;
    setBusy(true);
    setDraft({ ...draft, attempted: true });
    onBlocked(position, true);
    try {
      const response = await fetch(
        `/api/sessions/${sessionID}/sets/${draft.id}`,
        {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ exercise_position: position, weight, reps }),
        },
      );
      const body = await response.json();
      if (!response.ok) {
        setFields(body.error?.fields ?? {});
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
      finish();
    } catch {
      setError(
        "Could not confirm the save. Retry Save using the same set, or discard it.",
      );
    } finally {
      setBusy(false);
    }
  }
  async function remove(id: string) {
    if (busy || disabled) return;
    setBusy(true);
    setPendingDelete(id);
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
      finish();
    } catch {
      setError("Could not delete set. Please try again.");
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="mt-5 space-y-3">
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
                disabled={disabled || busy || !!draft || !!pendingDelete}
                onClick={() =>
                  edit({
                    id: set.id,
                    weight: set.weight,
                    reps: String(set.reps),
                    saved: true,
                    attempted: false,
                  })
                }
              >
                Edit set {index + 1}
              </Button>
              <Button
                variant="outline"
                disabled={disabled || busy || !!draft || !!pendingDelete}
                onClick={() => void remove(set.id)}
              >
                Delete set {index + 1}
              </Button>
            </li>
          ))}
        </ol>
      )}
      {draft ? (
        <form
          className="space-y-3 rounded-xl border border-border p-3"
          noValidate
          onSubmit={(event) => {
            event.preventDefault();
            void save();
          }}
        >
          <p className="text-sm font-medium">
            {draft.saved ? "Edit set" : "New set"}
          </p>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label
                htmlFor={`${formID}-weight`}
                className="mb-2 block text-sm"
              >
                Weight (kg)
              </label>
              <input
                id={`${formID}-weight`}
                type="text"
                inputMode="decimal"
                value={draft.weight}
                disabled={busy || disabled}
                onChange={(e) => setDraft({ ...draft, weight: e.target.value })}
                aria-invalid={!!fields.weight}
                aria-describedby={
                  fields.weight ? `${formID}-weight-error` : undefined
                }
                className="min-h-12 w-full min-w-0 rounded-lg border border-input bg-background px-3 text-base focus-visible:outline-2 focus-visible:outline-ring"
              />
              {fields.weight && (
                <p
                  id={`${formID}-weight-error`}
                  className="mt-1 text-sm text-destructive"
                >
                  {fields.weight}
                </p>
              )}
            </div>
            <div>
              <label htmlFor={`${formID}-reps`} className="mb-2 block text-sm">
                Reps
              </label>
              <input
                id={`${formID}-reps`}
                type="text"
                inputMode="numeric"
                value={draft.reps}
                disabled={busy || disabled}
                onChange={(e) => setDraft({ ...draft, reps: e.target.value })}
                aria-invalid={!!fields.reps}
                aria-describedby={
                  fields.reps ? `${formID}-reps-error` : undefined
                }
                className="min-h-12 w-full min-w-0 rounded-lg border border-input bg-background px-3 text-base focus-visible:outline-2 focus-visible:outline-ring"
              />
              {fields.reps && (
                <p
                  id={`${formID}-reps-error`}
                  className="mt-1 text-sm text-destructive"
                >
                  {fields.reps}
                </p>
              )}
            </div>
          </div>
          <div className="flex flex-wrap gap-3">
            <Button type="submit" disabled={busy || disabled}>
              {busy ? "Saving…" : "Save set"}
            </Button>
            <Button
              type="button"
              variant="outline"
              disabled={busy || disabled || (draft.saved && draft.attempted)}
              onClick={() => {
                if (!draft.saved && draft.attempted) void remove(draft.id);
                else finish();
              }}
            >
              {draft.saved ? "Cancel edit" : "Discard set"}
            </Button>
          </div>
          {draft.saved && draft.attempted && (
            <p className="text-xs text-muted-foreground">
              Retry Save to resolve this edit before cancelling it.
            </p>
          )}
          {!draft.saved && draft.attempted && (
            <p className="text-xs text-muted-foreground">
              Discard also removes this set if a save reached the server.
            </p>
          )}
        </form>
      ) : (
        <Button
          variant="outline"
          disabled={busy || disabled || !!error}
          onClick={() =>
            edit({
              id: crypto.randomUUID(),
              weight: "",
              reps: "",
              saved: false,
              attempted: false,
            })
          }
        >
          Add set
        </Button>
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
              onClick={() => void remove(pendingDelete)}
            >
              Retry delete
            </Button>
          )}
        </Alert>
      )}
    </div>
  );
}
