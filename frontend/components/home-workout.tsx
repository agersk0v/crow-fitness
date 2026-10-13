"use client";
import { useEffect, useRef, useState, useId } from "react";
import { toast } from "sonner";
import { ActiveSession } from "@/components/active-session";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";

export function HomeWorkout({ resetEnabled }: { resetEnabled: boolean }) {
  const [version, setVersion] = useState(0);
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const dialog = useRef<HTMLDialogElement>(null);
  const pending = useRef(false);
  const title = useId();
  useEffect(() => {
    if (confirming) dialog.current?.showModal();
    else dialog.current?.close();
  }, [confirming]);
  async function reset() {
    if (pending.current) return;
    pending.current = true;
    setBusy(true);
    setError("");
    try {
      const response = await fetch("/api/dev/reset", {
        method: "POST",
        headers: { "X-Confirm-Reset": "replace-workout-data" },
      });
      if (!response.ok) throw new Error();
      // The server commits before returning success. Reload the active-session
      // component so its captured old ID cannot survive replacement.
      setVersion((value) => value + 1);
      setConfirming(false);
      toast.success(
        "Dummy data ready: 4 templates, 48 completed workouts, 1,032 completed sets, and one active session.",
      );
    } catch {
      setError(
        "Could not confirm the reset. Refresh to check the current data before trying again.",
      );
    } finally {
      pending.current = false;
      setBusy(false);
    }
  }
  return (
    <>
      <ActiveSession key={version} />
      {resetEnabled && (
        <section
          className="space-y-3 rounded-xl border border-border p-5"
          aria-label="Testing data"
        >
          <p className="text-sm text-muted-foreground">
            Testing tool: replaces your workout data with your four routines and
            dummy history.
          </p>
          <Button
            type="button"
            variant="destructive"
            className="h-auto min-h-11 whitespace-normal"
            disabled={busy}
            onClick={() => {
              setError("");
              setConfirming(true);
            }}
          >
            Reset database and add dummy data
          </Button>
          <dialog
            ref={dialog}
            aria-labelledby={title}
            className="m-auto max-h-[90dvh] w-[calc(100%-2rem)] max-w-lg overflow-y-auto rounded-2xl border border-border bg-card p-6 text-foreground backdrop:bg-black/60"
            onCancel={(event) => {
              if (pending.current) event.preventDefault();
              else setConfirming(false);
            }}
            onClose={() => setConfirming(false)}
          >
            <h2 id={title} className="text-xl font-semibold">
              Replace all workout data?
            </h2>
            <p className="my-4 text-muted-foreground">
              This will remove your existing exercises, templates, active
              session, completed workouts, and sets. It will create Upper 1,
              Lower 1, Upper 2, Lower 2, 48 dummy completed workouts, 1,032
              completed sets, and one active session. This cannot be undone.
            </p>
            {error && (
              <Alert variant="destructive" className="mb-4">
                <AlertTitle>Reset could not be confirmed</AlertTitle>
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}
            <div className="flex flex-wrap gap-3">
              <Button
                type="button"
                variant="outline"
                autoFocus
                disabled={busy}
                onClick={() => setConfirming(false)}
              >
                Cancel
              </Button>
              <Button
                type="button"
                variant="destructive"
                disabled={busy}
                onClick={() => void reset()}
              >
                {busy ? "Replacing data…" : "Confirm reset and seed"}
              </Button>
            </div>
          </dialog>
        </section>
      )}
    </>
  );
}
