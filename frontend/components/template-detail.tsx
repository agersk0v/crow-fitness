"use client";
import Link from "next/link";
import { useEffect, useId, useRef, useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import { TemplateForm } from "@/components/template-form";
import { useDraftNavigation } from "@/hooks/use-draft-navigation";
import type { Exercise } from "@/lib/exercises";
import type { Template } from "@/lib/templates";

export function TemplateDetail({ id }: { id: string }) {
  const router = useRouter();
  const modal = useRef<HTMLDialogElement>(null);
  const dialogID = useId();
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
  const [saving, setSaving] = useState(false);
  const [version, setVersion] = useState(0);
  const [copied, setCopied] = useState<Template | null>(null);
  const canLeave = useDraftNavigation(dirty, saving || busy);
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
    if (pending.current || saving) return;
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
        // Keep the original draft intact. Opening the copy is an ordinary guarded link.
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
      <h1 className="break-words text-3xl font-semibold">
        {template.archived ? template.name : "Edit template"}
      </h1>
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
          <Button disabled={busy} onClick={() => void mutate("restore")}>
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
              </li>
            ))}
          </ol>
        </>
      ) : (
        <>
          <div className="my-5 space-y-3">
            <Button
              variant="outline"
              disabled={busy || saving}
              onClick={() => {
                setCopyName(`${template.name} (copy)`);
                setNameError("");
                setActionError("");
                setCopying(true);
              }}
            >
              Copy workout
            </Button>
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
            initial={template}
            exercises={exercises}
            disabled={busy}
            onDirtyChange={setDirty}
            onSavingChange={setSaving}
            onSaved={(saved) => {
              setDirty(false);
              setTemplate(saved);
              setVersion((value) => value + 1);
            }}
            onCancel={() => {
              if (canLeave()) router.push("/templates");
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
            disabled={busy}
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
            <Button type="submit" disabled={busy}>
              {busy ? "Copying…" : "Create copy"}
            </Button>
            <Button
              type="button"
              variant="outline"
              disabled={busy}
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
