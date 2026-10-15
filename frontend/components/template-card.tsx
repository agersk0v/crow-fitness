"use client";
import { ChevronDown } from "lucide-react";
import { useEffect, useId, useRef, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { StartSession } from "@/components/start-session";
import {
  TemplateDetail,
  type TemplateEditorState,
} from "@/components/template-detail";
import type { Template, TemplateSummary } from "@/lib/templates";

export function TemplateCard({
  template,
  onArchived,
  onSaved,
  onCopied,
  onRestored,
  onEditorStateChange,
  canLeave,
  starting,
  onStartingChange,
}: {
  template: TemplateSummary;
  onArchived: (id: string) => void;
  onSaved: (template: Template) => void;
  onCopied: (template: Template) => void;
  onRestored: (template: Template) => void;
  onEditorStateChange: (id: string, state: TemplateEditorState | null) => void;
  canLeave: () => boolean;
  starting: boolean;
  onStartingChange: (busy: boolean) => void;
}) {
  const [open, setOpen] = useState(false);
  const [visited, setVisited] = useState(false);
  const [editor, setEditor] = useState<TemplateEditorState>({
    dirty: false,
    busy: false,
  });
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    onEditorStateChange(template.id, {
      dirty: editor.dirty,
      busy: editor.busy || busy,
    });
  }, [editor, busy, template.id, onEditorStateChange]);
  useEffect(
    () => () => onEditorStateChange(template.id, null),
    [template.id, onEditorStateChange],
  );
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
    if (pending.current || editor.busy || starting) return;
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
    <li className="min-w-0 overflow-hidden rounded-2xl border border-border bg-card">
      <h3>
        <button
          type="button"
          id={`${title}-trigger`}
          aria-expanded={open}
          aria-controls={`${title}-panel`}
          className="flex w-full items-center justify-between gap-4 p-5 text-left hover:bg-primary/5 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring"
          onClick={() => {
            setOpen(!open);
            setVisited(true);
          }}
        >
          <span className="min-w-0">
            <span className="block break-words font-medium text-highlight">
              {template.name}
              {template.archived ? " (Archived)" : ""}
            </span>
            {template.description && (
              <span className="mt-2 block whitespace-pre-wrap break-words text-sm font-normal text-muted-foreground">
                {template.description}
              </span>
            )}
          </span>
          <ChevronDown
            aria-hidden="true"
            className={`size-5 shrink-0 ${open ? "rotate-180" : ""}`}
          />
        </button>
      </h3>
      {!template.archived && (
        <div className="flex flex-wrap items-start gap-3 px-5 pb-4">
          <StartSession
            templateID={template.id}
            compact
            beforeStart={canLeave}
            disabled={starting || editor.busy || busy}
            onBusyChange={onStartingChange}
          />
          <Button
            disabled={starting || editor.busy || busy}
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
      <div
        id={`${title}-panel`}
        role="region"
        aria-labelledby={`${title}-trigger`}
        hidden={!open}
        className="border-t border-border p-5"
      >
        {visited && (
          <TemplateDetail
            id={template.id}
            inline
            disabled={starting || busy}
            onStateChange={setEditor}
            onSaved={onSaved}
            onCopied={onCopied}
            onRestored={onRestored}
          />
        )}
      </div>
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
        {editor.dirty && (
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
    </li>
  );
}
