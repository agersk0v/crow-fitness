"use client";
import Link from "next/link";
import { useEffect, useId, useRef, useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import { ArchiveTemplate } from "@/components/archive-template";
import { TemplateForm } from "@/components/template-form";
import type { Exercise } from "@/lib/exercises";
import type { Template } from "@/lib/templates";

export function TemplateDetail({ id }: { id: string }) {
  const router = useRouter();
  const modal = useRef<HTMLDialogElement>(null);
  const dialogID = useId();
  const editFormID = useId();
  const [template, setTemplate] = useState<Template | null>(null);
  const [exercises, setExercises] = useState<Exercise[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [retry, setRetry] = useState(0);
  const [copying, setCopying] = useState(false);
  const [copyName, setCopyName] = useState("");
  const [nameError, setNameError] = useState("");
  const [actionError, setActionError] = useState("");
  const [busy, setBusy] = useState(false);
  const pending = useRef(false);
  const [dirty, setDirty] = useState(false);
  const [archiving, setArchiving] = useState(false);
  const [saving, setSaving] = useState(false);
  const [version, setVersion] = useState(0);
  const [copied, setCopied] = useState<Template | null>(null);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    async function load() {
      try {
        const response = await fetch(
          `/api/templates/${encodeURIComponent(id)}`,
          { cache: "no-store", signal: controller.signal },
        );
        if (!response.ok)
          throw new Error(
            response.status === 404
              ? "This template could not be found."
              : "Could not load this template. Please try again.",
          );
        const body: Template = await response.json();
        const library = await fetch("/api/exercises", {
          cache: "no-store",
          signal: controller.signal,
        });
        if (!library.ok)
          throw new Error("Could not load exercises. Please try again.");
        const libraryBody = await library.json();
        if (!controller.signal.aborted) {
          setTemplate(body);
          setExercises(libraryBody.items);
        }
      } catch (e) {
        if (!controller.signal.aborted)
          setError(
            e instanceof Error ? e.message : "Could not load this template.",
          );
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    }
    void load();
    return () => controller.abort();
  }, [id, retry]);
  useEffect(() => {
    if (copying) modal.current?.showModal();
    else modal.current?.close();
  }, [copying]);
  async function mutate(kind: "duplicate" | "restore") {
    if (pending.current || saving || archiving) return;
    pending.current = true;
    setBusy(true);
    setActionError("");
    setNameError("");
    try {
      const response = await fetch(`/api/templates/${id}/${kind}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body:
          kind === "duplicate" ? JSON.stringify({ name: copyName }) : undefined,
      });
      const body = await response.json();
      if (!response.ok) {
        setActionError(
          body.error?.message ?? "Could not change template. Please try again.",
        );
        setNameError(body.error?.fields?.name ?? "");
        return;
      }
      if (kind === "duplicate") {
        setCopied(body);
        setCopying(false);
        toast.success(`${body.name} copied.`);
        // Keep the original draft intact until the user saves or leaves this editor.
      } else {
        setTemplate(body);
        toast.success(`${body.name} restored.`);
      }
    } catch {
      setActionError("Could not change template. Please try again.");
    } finally {
      pending.current = false;
      setBusy(false);
    }
  }
  if (loading) return <p role="status">Loading template…</p>;
  if (error)
    return (
      <Alert variant="destructive">
        <AlertTitle>Could not open template</AlertTitle>
        <AlertDescription>{error}</AlertDescription>
        <Button
          className="col-start-2 mt-3"
          onClick={() => setRetry(retry + 1)}
        >
          Try again
        </Button>
      </Alert>
    );
  if (!template) return null;
  return (
    <>
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div className="flex min-w-0 flex-wrap items-center gap-3">
          <h1 className="break-words text-3xl font-semibold">
            {template.archived ? template.name : "Edit template"}
          </h1>
          {!template.archived && dirty && (
            <span
              role="status"
              className="text-xs font-medium text-red-600 dark:text-red-400"
            >
              Unsaved changes
            </span>
          )}
        </div>
        {!template.archived && (
          <Button
            type="submit"
            form={editFormID}
            className="min-h-12"
            disabled={saving || busy || archiving || exercises.length === 0}
          >
            {saving ? "Saving…" : "Save template"}
          </Button>
        )}
      </div>
      {template.archived ? (
        <>
          {template.description && (
            <p className="mt-4 whitespace-pre-wrap break-words text-muted-foreground">
              {template.description}
            </p>
          )}
          <p className="my-4 text-muted-foreground">
            Archived — restore this template before editing, copying, or
            starting a workout. Existing workout history remains intact.
          </p>
          <Button
            disabled={busy || archiving}
            onClick={() => void mutate("restore")}
          >
            {busy ? "Restoring…" : "Restore template"}
          </Button>
          {actionError && (
            <Alert variant="destructive" className="mt-4">
              <AlertTitle>Could not restore template</AlertTitle>
              <AlertDescription>{actionError}</AlertDescription>
            </Alert>
          )}
          <ol className="mt-6 space-y-4">
            {template.exercises.map((entry) => (
              <li
                key={entry.position}
                className="rounded-xl border border-border bg-card p-5"
              >
                <h2 className="break-words font-semibold">
                  {entry.position}. {entry.name}
                </h2>
                <p className="mt-2">
                  {entry.set_count} working sets · Rep target:{" "}
                  {entry.rep_min === null
                    ? "Not specified"
                    : `${entry.rep_min}–${entry.rep_max}`}{" "}
                  · RIR: {entry.target_rir ?? "Not specified"}
                </p>
                {!!entry.alternatives?.length && (
                  <div className="mt-3 space-y-2 text-sm text-muted-foreground">
                    <p>Default: {entry.name}</p>
                    {entry.alternatives.map((c) => (
                      <p key={c.exercise_id}>
                        Or {c.name} · {c.set_count} sets
                        {c.rep_min !== null &&
                          ` · ${c.rep_min}–${c.rep_max} reps`}
                        {c.target_rir !== null && ` · RIR ${c.target_rir}`}
                      </p>
                    ))}
                  </div>
                )}
              </li>
            ))}
          </ol>
        </>
      ) : (
        <>
          <div className="my-5 space-y-3">
            <div className="flex flex-wrap gap-3">
              <Button
                variant="outline"
                disabled={busy || saving || archiving}
                onClick={() => {
                  setCopyName(`${template.name} (copy)`);
                  setNameError("");
                  setActionError("");
                  setCopying(true);
                }}
              >
                Copy workout
              </Button>
              <ArchiveTemplate
                template={template}
                dirty={dirty}
                disabled={busy || saving}
                onBusyChange={setArchiving}
                onArchived={() => {
                  router.push("/templates");
                }}
              />
            </div>
            {dirty && (
              <p className="text-sm text-muted-foreground">
                Copy workout uses the last saved configuration. Save first to
                include your unsaved changes.
              </p>
            )}
            {copied && (
              <Link
                href={`/templates/${copied.id}`}
                className="block break-words text-highlight underline"
              >
                Edit copied workout: {copied.name}
              </Link>
            )}
          </div>
          <TemplateForm
            key={version}
            id={editFormID}
            externalSave
            initial={template}
            exercises={exercises}
            disabled={busy || archiving}
            onDirtyChange={setDirty}
            onSavingChange={setSaving}
            onSaved={(saved) => {
              setDirty(false);
              setSaving(false);
              setTemplate(saved);
              setVersion((value) => value + 1);
            }}
          />
        </>
      )}
      <dialog
        ref={modal}
        aria-labelledby={`${dialogID}-title`}
        onCancel={(event) => {
          if (pending.current) event.preventDefault();
          else setCopying(false);
        }}
        onClose={() => setCopying(false)}
        className="m-auto max-h-[90dvh] w-[calc(100%-2rem)] max-w-lg overflow-y-auto rounded-2xl border border-border bg-card p-5 text-foreground backdrop:bg-black/60"
      >
        <h2 id={`${dialogID}-title`} className="text-xl font-semibold">
          Copy workout
        </h2>
        <p className="mt-3 text-sm text-muted-foreground">
          This creates an independent copy of the last saved template.{" "}
          {dirty
            ? "Unsaved edits are not included; your current draft will stay here."
            : ""}
        </p>
        <form
          className="mt-4 space-y-4"
          noValidate
          onSubmit={(event: FormEvent<HTMLFormElement>) => {
            event.preventDefault();
            void mutate("duplicate");
          }}
        >
          <label
            htmlFor={`${dialogID}-name`}
            className="block text-sm font-medium"
          >
            Name for the copy
          </label>
          <input
            autoFocus
            id={`${dialogID}-name`}
            value={copyName}
            disabled={busy || archiving}
            onChange={(event) => setCopyName(event.target.value)}
            aria-invalid={Boolean(nameError)}
            className="min-h-12 w-full rounded-lg border border-input bg-background px-3"
          />
          {nameError && <p className="text-sm text-destructive">{nameError}</p>}
          {actionError && (
            <Alert variant="destructive">
              <AlertTitle>Could not copy workout</AlertTitle>
              <AlertDescription>{actionError}</AlertDescription>
            </Alert>
          )}
          <div className="flex flex-wrap gap-3">
            <Button type="submit" disabled={busy || archiving}>
              {busy ? "Copying…" : "Create copy"}
            </Button>
            <Button
              type="button"
              variant="outline"
              disabled={busy || archiving}
              onClick={() => setCopying(false)}
            >
              Cancel
            </Button>
          </div>
        </form>
      </dialog>
    </>
  );
}
