"use client";
import Link from "next/link";
import { useEffect, useId, useRef, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { StartSession } from "@/components/start-session";
import type { TemplateSummary } from "@/lib/templates";

export function TemplateCard({
  template,
  onArchived,
}: {
  template: TemplateSummary;
  onArchived: (id: string) => void;
}) {
  const modal = useRef<HTMLDialogElement>(null);
  const title = useId();
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const pending = useRef(false);
  const [error, setError] = useState("");
  useEffect(() => {
    if (confirming) modal.current?.showModal();
    else modal.current?.close();
  }, [confirming]);
  async function archive() {
    if (pending.current) return;
    pending.current = true;
    setBusy(true);
    setError("");
    try {
      const response = await fetch(`/api/templates/${template.id}/archive`, {
        method: "POST",
      });
      if (!response.ok) throw new Error();
      setConfirming(false);
      onArchived(template.id);
      toast.success(`${template.name} archived.`);
    } catch {
      setError("Could not archive this template. Please try again.");
    } finally {
      pending.current = false;
      setBusy(false);
    }
  }
  return (
    <li className="relative min-w-0 rounded-2xl border border-border bg-card px-5 py-4 hover:bg-primary/5">
      <Link
        href={`/templates/${template.id}`}
        aria-label={`${template.archived ? "View archived" : "Edit"} ${template.name}`}
        className="break-words font-medium text-highlight after:absolute after:inset-0 after:content-[''] hover:underline focus-visible:outline-none focus-visible:after:outline-2 focus-visible:after:outline-offset-[-2px] focus-visible:after:outline-ring"
      >
        {template.name}
        {template.archived ? " (Archived)" : ""} →
      </Link>
      {template.description && (
        <p className="mt-2 whitespace-pre-wrap break-words text-sm text-muted-foreground">
          {template.description}
        </p>
      )}
      {!template.archived && (
        <div className="mt-3 flex flex-wrap items-start gap-3">
          <StartSession templateID={template.id} compact />
          <Button
            className="relative z-10"
            variant="outline"
            onClick={() => {
              setError("");
              setConfirming(true);
            }}
            aria-label={`Delete ${template.name}`}
          >
            Delete
          </Button>
        </div>
      )}
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
    </li>
  );
}
