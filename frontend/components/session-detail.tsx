"use client";
import { useCallback, useEffect, useId, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import {
  ExerciseReferences,
  usePersonalBests,
  usePreviousPerformance,
} from "@/components/previous-performance";
import { SessionSets } from "@/components/session-sets";
import type { Session } from "@/lib/sessions";

export function SessionDetail({ id }: { id: string }) {
  const router = useRouter();
  const modal = useRef<HTMLDialogElement>(null);
  const dialogID = useId();
  const blockers = useRef(new Set<number>());
  const [blocked, setBlocked] = useState(false);
  const [session, setSession] = useState<Session | null>(null);
  const selectionKey =
    session?.exercises
      .map((e) => `${e.position}:${e.selection_revision ?? 0}`)
      .join(",") ?? "";
  const previous = usePreviousPerformance(id, selectionKey);
  const bests = usePersonalBests(id, selectionKey);
  const onSelected = useCallback((saved: Session, position: number) => {
    setSession(
      (current) =>
        current && {
          ...current,
          exercises: current.exercises.map((e) =>
            e.position === position
              ? saved.exercises.find((next) => next.position === position)!
              : e,
          ),
          sets: [
            ...current.sets.filter((s) => s.exercise_position !== position),
            ...saved.sets.filter((s) => s.exercise_position === position),
          ],
        },
    );
    blockers.current.delete(position);
    setBlocked(blockers.current.size > 0);
  }, []);
  const [loading, setLoading] = useState(true);
  const [missing, setMissing] = useState(false);
  const [error, setError] = useState("");
  const [retry, setRetry] = useState(0);
  const [confirming, setConfirming] = useState(false);
  const [missingSets, setMissingSets] = useState<MissingSets[]>([]);
  const [confirmMissing, setConfirmMissing] = useState(false);
  const [busy, setBusy] = useState<"cancel" | "complete" | null>(null);
  const [actionError, setActionError] = useState("");
  const onBlocked = useCallback((position: number, isBlocked: boolean) => {
    if (isBlocked) blockers.current.add(position);
    else blockers.current.delete(position);
    setBlocked(blockers.current.size > 0);
  }, []);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setMissing(false);
    setError("");
    async function load() {
      try {
        const response = await fetch(
          `/api/sessions/${encodeURIComponent(id)}`,
          { cache: "no-store", signal: controller.signal },
        );
        if (response.status === 404) {
          if (!controller.signal.aborted) setMissing(true);
          return;
        }
        if (!response.ok) throw new Error("Could not load session.");
        const body: Session = await response.json();
        if (!controller.signal.aborted) setSession(body);
      } catch {
        if (!controller.signal.aborted)
          setError("Could not load this session. Please try again.");
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    }
    void load();
    return () => controller.abort();
  }, [id, retry]);
  useEffect(() => {
    if (confirming) modal.current?.showModal();
    else modal.current?.close();
  }, [confirming]);
  async function end(action: "cancel" | "complete", allowIncomplete = false) {
    if (busy || (action === "complete" && blockers.current.size)) return;
    setBusy(action);
    setActionError("");
    try {
      if (action === "complete" && !allowIncomplete) {
        const current = await fetch(`/api/sessions/${id}`, {
          cache: "no-store",
        });
        if (!current.ok) throw new Error("Could not check session.");
        const saved: Session = await current.json();
        const missing = saved.exercises
          .map((entry) => ({
            name: entry.name,
            position: entry.position,
            planned: entry.set_count,
            saved: saved.sets.filter(
              (set) => set.exercise_position === entry.position,
            ).length,
          }))
          .filter((entry) => entry.saved < entry.planned);
        if (missing.length) {
          setMissingSets(missing);
          setConfirmMissing(true);
          return;
        }
      }
      const response = await fetch(`/api/sessions/${id}/${action}`, {
        method: "POST",
      });
      if (!response.ok) {
        const body = await response.json();
        setActionError(
          body.error?.message ?? "Could not end session. Please try again.",
        );
        return;
      }
      setConfirming(false);
      setConfirmMissing(false);
      toast.success(
        action === "cancel" ? "Session cancelled." : "Session completed.",
      );
      router.push("/");
      router.refresh();
    } catch {
      setActionError(
        "Could not end session. Please try again. Your session has not been replaced.",
      );
    } finally {
      setBusy(null);
    }
  }
  if (loading) return <p role="status">Loading session…</p>;
  if (error)
    return (
      <Alert variant="destructive">
        <AlertTitle>Could not open session</AlertTitle>
        <AlertDescription>{error}</AlertDescription>
        <Button
          className="col-start-2 mt-3"
          onClick={() => setRetry(retry + 1)}
        >
          Try again
        </Button>
      </Alert>
    );
  if (missing)
    return (
      <div className="space-y-4">
        <h1 className="text-3xl font-semibold">Session no longer active</h1>
        <p className="text-muted-foreground">
          This session has ended or could not be found.
        </p>
        <Link href="/" className="text-highlight underline">
          Return home
        </Link>
      </div>
    );
  if (!session) return null;
  return (
    <>
      <p className="text-sm font-medium text-highlight">Active workout</p>
      <h1 className="mt-2 break-words text-4xl font-semibold">
        {session.name}
      </h1>
      <p className="mt-3 text-sm text-muted-foreground">
        Started {new Date(session.started_at).toLocaleString()}
      </p>
      <p className="my-6 text-muted-foreground">
        Your workout is saved for you to continue later. These targets are the
        configuration you started with. Saved sets stay in this active session
        until you complete the workout. Cancellation discards them.
      </p>
      <ol className="space-y-4">
        {session.exercises.map((entry) => (
          <li
            key={entry.position}
            className="rounded-2xl border border-border bg-card p-5"
          >
            <SessionSets
              key={`${id}-${entry.position}-${entry.selection_revision ?? 0}`}
              exercise={entry}
              onSelected={onSelected}
              sessionID={id}
              position={entry.position}
              plannedCount={entry.set_count}
              initial={session.sets.filter(
                (set) => set.exercise_position === entry.position,
              )}
              disabled={!!busy}
              previousWeight={
                previous.items.find(
                  (item) => item.exercise_id === entry.exercise_id,
                )?.occurrences[0]?.sets[0]?.weight
              }
              onBlocked={onBlocked}
            >
              <dl className="mt-4 grid grid-cols-2 gap-3 text-sm">
                <dt>Working sets</dt>
                <dd>{entry.set_count}</dd>
                <dt>Rep target</dt>
                <dd>
                  {entry.rep_min === null
                    ? "Not specified"
                    : entry.rep_min === entry.rep_max
                      ? entry.rep_min
                      : `${entry.rep_min}–${entry.rep_max}`}
                </dd>
                <dt>Target RIR</dt>
                <dd>{entry.target_rir ?? "Not specified"}</dd>
              </dl>
              <ExerciseReferences
                label={entry.name}
                previous={previous.items.find(
                  (item) => item.exercise_id === entry.exercise_id,
                )}
                best={bests.items.find(
                  (item) => item.exercise_id === entry.exercise_id,
                )}
                previousState={previous}
                bestState={bests}
              />
            </SessionSets>
          </li>
        ))}
      </ol>
      {blocked && (
        <p role="status" className="mt-5 text-sm text-muted-foreground">
          Save edits, clear unfinished inputs, and resolve failed changes before
          completing.
        </p>
      )}
      <div className="my-6 flex flex-wrap gap-3">
        <Button
          disabled={!!busy || blocked}
          onClick={() => void end("complete")}
        >
          {busy === "complete" ? "Completing…" : "Complete session"}
        </Button>
        <Button
          variant="outline"
          disabled={!!busy}
          onClick={() => {
            setActionError("");
            setConfirming(true);
          }}
        >
          Cancel session
        </Button>
        <Link
          href="/"
          className="inline-flex min-h-11 items-center px-3 text-highlight underline"
        >
          Continue later
        </Link>
      </div>
      {confirmMissing && (
        <IncompleteSessionDialog
          missing={missingSets}
          busy={!!busy}
          error={actionError}
          onCancel={() => {
            setConfirmMissing(false);
            setActionError("");
          }}
          onConfirm={() => void end("complete", true)}
        />
      )}
      {!confirming && !confirmMissing && actionError && (
        <Alert variant="destructive">
          <AlertTitle>Could not end session</AlertTitle>
          <AlertDescription>{actionError}</AlertDescription>
        </Alert>
      )}
      <dialog
        ref={modal}
        aria-labelledby={`${dialogID}-title`}
        onCancel={(event) => {
          event.preventDefault();
          if (!busy) setConfirming(false);
        }}
        onClose={() => setConfirming(false)}
        className="m-auto max-h-[90dvh] w-[calc(100%-2rem)] max-w-lg overflow-y-auto rounded-2xl border border-border bg-card p-5 text-foreground shadow-xl backdrop:bg-black/60"
      >
        <h2
          id={`${dialogID}-title`}
          className="break-words text-xl font-semibold"
        >
          Cancel {session.name}?
        </h2>
        <p className="mt-4 text-sm text-muted-foreground">
          This discards your active session and all its saved sets. These sets
          will not appear on the Sets page. You can start another workout
          afterward.
        </p>
        {actionError && (
          <Alert variant="destructive" className="mt-4">
            <AlertTitle>Could not cancel session</AlertTitle>
            <AlertDescription>{actionError}</AlertDescription>
          </Alert>
        )}
        <div className="mt-4 flex flex-wrap gap-3">
          <Button
            autoFocus
            variant="outline"
            disabled={!!busy}
            onClick={() => setConfirming(false)}
          >
            Keep session
          </Button>
          <Button disabled={!!busy} onClick={() => void end("cancel")}>
            {busy === "cancel" ? "Cancelling…" : "Confirm cancel"}
          </Button>
        </div>
      </dialog>
    </>
  );
}

type MissingSets = {
  name: string;
  position: number;
  planned: number;
  saved: number;
};
function IncompleteSessionDialog({
  missing,
  busy,
  error,
  onCancel,
  onConfirm,
}: {
  missing: MissingSets[];
  busy: boolean;
  error: string;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  const title = useId();
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
      aria-labelledby={title}
      onCancel={(event) => {
        event.preventDefault();
        if (!busy) onCancel();
      }}
      className="m-auto max-h-[90dvh] w-[calc(100%-2rem)] max-w-lg overflow-y-auto rounded-2xl border border-border bg-card p-5 text-foreground shadow-xl backdrop:bg-black/60"
    >
      <h2 id={title} className="text-xl font-semibold">
        Complete with missing sets?
      </h2>
      <p className="mt-3 text-sm text-muted-foreground">
        These planned sets have not been saved. You can return to your workout
        or complete it anyway.
      </p>
      <ul className="my-4 space-y-3">
        {missing.map((entry) => (
          <li key={entry.position}>
            <p className="break-words font-medium">
              {entry.name} (exercise {entry.position})
            </p>
            <p className="text-sm text-muted-foreground">
              {entry.saved + 1 === entry.planned
                ? `Set ${entry.planned}`
                : `Sets ${entry.saved + 1}–${entry.planned}`}{" "}
              missing ({entry.saved}/{entry.planned} saved).
            </p>
          </li>
        ))}
      </ul>
      {error && (
        <p role="alert" className="mb-4 text-sm text-destructive">
          {error}
        </p>
      )}
      <div className="flex flex-wrap gap-3">
        <Button autoFocus variant="outline" disabled={busy} onClick={onCancel}>
          Return to session
        </Button>
        <Button disabled={busy} onClick={onConfirm}>
          {busy ? "Completing…" : "Complete anyway"}
        </Button>
      </div>
    </dialog>
  );
}
