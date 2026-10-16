"use client";
import { useEffect, useId, useRef, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import type { TemplateSummary } from "@/lib/templates";
export function ArchiveTemplate({
  template,
  dirty,
  disabled,
  onArchived,
  onBusyChange,
}: {
  template: TemplateSummary;
  dirty: boolean;
  disabled: boolean;
  onArchived: () => void;
  onBusyChange: (busy: boolean) => void;
}) {
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    onBusyChange(busy);
  }, [busy, onBusyChange]);
  const modal = useRef<HTMLDialogElement>(null);
  const title = useId();
  const [confirming, setConfirming] = useState(false);
  const pending = useRef(false);
  const [error, setError] = useState("");
  useEffect(() => {
    if (confirming) modal.current?.showModal();
    else modal.current?.close();
  }, [confirming]);
  async function archive() {
    if (pending.current || disabled) return;
    pending.current = true;
    setBusy(true);
    setError("");
    let archived = false;
    try {
      const response = await fetch(`/api/templates/${template.id}/archive`, {
        method: "POST",
      });
      if (!response.ok) throw new Error();
      archived = true;
      if (!mounted.current) return;
      setConfirming(false);
      onArchived();
      toast.success(`${template.name} archived.`);
    } catch {
      if (mounted.current)
        setError("Could not archive this template. Please try again.");
    } finally {
      // Keep the archived editor disabled until navigation unmounts it.
      if (!archived && mounted.current) {
        pending.current = false;
        setBusy(false);
      }
    }
  }
  return (
    <>
      <Button
        variant="destructive"
        disabled={disabled || busy}
        onClick={() => {
          setError("");
          setConfirming(true);
        }}
      >
        Delete
      </Button>
      <dialog
        ref={modal}
        aria-labelledby={title}
        onCancel={(event) => {
          if (pending.current) event.preventDefault();
          else setConfirming(false);
        }}
        onClose={() => setConfirming(false)}
        className="m-auto max-h-[90dvh] w-[calc(100%-2rem)] max-w-lg overflow-y-auto rounded-2xl border border-border bg-card p-5 text-foreground backdrop:bg-black/60"
      >
        <h2 id={title} className="break-words text-xl font-semibold">
          Delete {template.name}?
        </h2>
        <p className="my-4 text-muted-foreground">
          This will only archive “{template.name}”, not permanently delete it.
          It will be hidden from Active and can be restored from Archived.
          Existing active sessions and completed workouts/sets stay intact.
        </p>
        {dirty && (
          <p className="mb-4 text-muted-foreground">
            Unsaved changes to this template will be discarded if you confirm
            archiving.
          </p>
        )}
        {error && (
          <Alert variant="destructive" className="mb-4">
            <AlertTitle>Could not archive template</AlertTitle>
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}
        <div className="flex flex-wrap gap-3">
          <Button
            autoFocus
            variant="outline"
            disabled={busy}
            onClick={() => setConfirming(false)}
          >
            Cancel
          </Button>
          <Button
            variant="destructive"
            disabled={busy}
            onClick={() => void archive()}
          >
            {busy ? "Archiving…" : "Confirm archive"}
          </Button>
        </div>
      </dialog>
    </>
  );
}
