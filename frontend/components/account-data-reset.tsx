"use client";
import { useEffect, useRef, useState, useId } from "react";
import { toast } from "sonner";
import { leaveAccount, useAuth } from "@/components/auth-boundary";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";

export function AccountDataReset() {
  const auth = useAuth();
  return auth.status === "signed-in" && !auth.revalidating ? (
    <ResetTool />
  ) : null;
}

function ResetTool() {
  const [fullReset, setFullReset] = useState(false);
  const [confirmation, setConfirmation] = useState("");
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
      const response = await fetch(
        fullReset ? "/api/dev/reset-all" : "/api/dev/reset",
        {
          method: "POST",
          headers: {
            "X-Confirm-Reset": fullReset
              ? "delete-all-data-and-users"
              : "replace-workout-data",
          },
        },
      );
      if (!response.ok) throw new Error();
      if (fullReset) {
        leaveAccount();
        return;
      }
      window.dispatchEvent(new Event("crow-active-session"));
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
      <section
        className="space-y-3 rounded-xl border border-border p-5"
        aria-label="Testing data"
      >
        <p className="text-sm text-muted-foreground">
          Development tools: replace your account’s workout data with dummy
          data, or delete all accounts and workout data.
        </p>
        <div className="flex flex-wrap gap-3">
          <Button
            type="button"
            variant="destructive"
            className="h-auto min-h-11 whitespace-normal"
            disabled={busy}
            onClick={() => {
              setError("");
              setFullReset(false);
              setConfirming(true);
            }}
          >
            Reset my workout data and add dummy data
          </Button>
          <Button
            type="button"
            variant="destructive"
            className="h-auto min-h-11 whitespace-normal"
            disabled={busy}
            onClick={() => {
              setError("");
              setFullReset(true);
              setConfirmation("");
              setConfirming(true);
            }}
          >
            Full reset: delete all data and users
          </Button>
        </div>
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
            {fullReset
              ? "Delete all data and users?"
              : "Replace your account’s workout data?"}
          </h2>
          {fullReset ? (
            <>
              <p className="my-4 text-muted-foreground">
                This deletes every account and all exercises, templates,
                workouts and sets, including unclaimed legacy data. Everyone
                will be signed out. No dummy data will be added. This cannot be
                undone.
              </p>
              <label className="block space-y-2 mb-4">
                Type DELETE ALL to confirm
                <input
                  className="block w-full rounded-md border border-border bg-background p-2"
                  value={confirmation}
                  onChange={(event) => setConfirmation(event.target.value)}
                  disabled={busy}
                  autoComplete="off"
                />
              </label>
            </>
          ) : (
            <p className="my-4 text-muted-foreground">
              This will remove the signed-in account’s existing exercises,
              templates, active session, completed workouts, and sets. It will
              create Upper 1, Lower 1, Upper 2, Lower 2, 48 dummy completed
              workouts, 1,032 completed sets, and one active session. This
              cannot be undone.
            </p>
          )}
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
              disabled={busy || (fullReset && confirmation !== "DELETE ALL")}
              onClick={() => void reset()}
            >
              {busy
                ? "Resetting…"
                : fullReset
                  ? "Delete all data and users"
                  : "Confirm reset and seed"}
            </Button>
          </div>
        </dialog>
      </section>
    </>
  );
}
