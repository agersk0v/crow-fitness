"use client";
import { useEffect, useId, useRef, useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import { TemplateForm } from "@/components/template-form";
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
  const [editing, setEditing] = useState(false);
  const [action, setAction] = useState<"duplicate" | "archive" | null>(null);
  const [copyName, setCopyName] = useState("");
  const [nameError, setNameError] = useState("");
  const [actionError, setActionError] = useState("");
  const [busy, setBusy] = useState(false);
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
    if (action) modal.current?.showModal();
    else modal.current?.close();
  }, [action]);
  function open(next: "duplicate" | "archive") {
    setCopyName(`${template?.name ?? ""} (copy)`);
    setNameError("");
    setActionError("");
    setAction(next);
  }
  async function mutate(kind: "duplicate" | "archive" | "restore") {
    if (busy) return;
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
        setAction(null);
        toast.success(`${body.name} created.`);
        router.push(`/templates/${body.id}`);
      } else {
        setTemplate(body);
        setAction(null);
        toast.success(
          `${body.name} ${kind === "archive" ? "archived" : "restored"}.`,
        );
      }
    } catch {
      setActionError("Could not change template. Please try again.");
    } finally {
      setBusy(false);
    }
  }
  if (loading) return <p role="status">Loading template…</p>;
  if (error)
    return (
      <Alert variant="destructive">
        <AlertTitle>Could not open template</AlertTitle>
        <AlertDescription>{error}</AlertDescription>
        <Button className="mt-3" onClick={() => setRetry(retry + 1)}>
          Try again
        </Button>
      </Alert>
    );
  if (!template) return null;
  if (editing)
    return (
      <>
        <h1 className="text-3xl font-semibold">Edit template</h1>
        <TemplateForm
          initial={template}
          exercises={exercises}
          onSaved={(saved) => {
            setTemplate(saved);
            setEditing(false);
          }}
          onCancel={() => {
            setEditing(false);
            setRetry(retry + 1);
          }}
        />
      </>
    );
  return (
    <>
      <h1 className="break-words text-4xl font-semibold">{template.name}</h1>
      {template.archived && (
        <p className="mt-3 rounded-lg border border-border bg-primary/5 p-3 text-sm">
          Archived — restore this template before editing or duplicating it.
          Exercise references are preserved.
        </p>
      )}
      <div className="my-5 flex flex-wrap gap-3">
        {template.archived ? (
          <Button disabled={busy} onClick={() => void mutate("restore")}>
            {busy ? "Restoring…" : "Restore template"}
          </Button>
        ) : (
          <>
            <Button onClick={() => setEditing(true)}>Edit template</Button>
            <Button variant="outline" onClick={() => open("duplicate")}>
              Duplicate template
            </Button>
            <Button variant="outline" onClick={() => open("archive")}>
              Archive template
            </Button>
          </>
        )}
      </div>
      {!action && actionError && (
        <Alert variant="destructive">
          <AlertTitle>Could not change template</AlertTitle>
          <AlertDescription>{actionError}</AlertDescription>
        </Alert>
      )}
      <p className="mb-6 text-muted-foreground">
        Targets apply to every working set of each exercise. Warm-up sets are
        excluded.
      </p>
      <ol className="space-y-4">
        {template.exercises.map((entry) => (
          <li
            key={entry.position}
            className="rounded-2xl border border-border bg-card p-5"
          >
            <h2 className="break-words text-xl font-semibold">
              {entry.position}. {entry.name}
            </h2>
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
          </li>
        ))}
      </ol>
      <dialog
        ref={modal}
        aria-labelledby={`${dialogID}-title`}
        onCancel={(event) => {
          event.preventDefault();
          if (!busy) setAction(null);
        }}
        onClose={() => setAction(null)}
        className="m-auto max-h-[90dvh] w-[calc(100%-2rem)] max-w-lg overflow-y-auto rounded-2xl border border-border bg-card p-5 text-foreground shadow-xl backdrop:bg-black/60"
      >
        <h2
          id={`${dialogID}-title`}
          className="break-words text-xl font-semibold"
        >
          {action === "duplicate"
            ? "Duplicate template"
            : `Archive ${template.name}?`}
        </h2>
        {action === "duplicate" ? (
          <form
            className="mt-4 space-y-4"
            onSubmit={(event: FormEvent<HTMLFormElement>) => {
              event.preventDefault();
              void mutate("duplicate");
            }}
            noValidate
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
              aria-describedby={nameError ? `${dialogID}-error` : undefined}
              className="min-h-12 w-full rounded-lg border border-input bg-background px-3 text-base focus-visible:outline-2 focus-visible:outline-ring"
            />
            {nameError && (
              <p id={`${dialogID}-error`} className="text-sm text-destructive">
                {nameError}
              </p>
            )}
            {actionError && (
              <Alert variant="destructive">
                <AlertTitle>Could not duplicate template</AlertTitle>
                <AlertDescription>{actionError}</AlertDescription>
              </Alert>
            )}
            <div className="flex flex-wrap gap-3">
              <Button type="submit" disabled={busy}>
                {busy ? "Duplicating…" : "Create copy"}
              </Button>
              <Button
                type="button"
                variant="outline"
                disabled={busy}
                onClick={() => setAction(null)}
              >
                Cancel
              </Button>
            </div>
          </form>
        ) : (
          <div className="mt-4 space-y-4">
            <p className="break-words text-sm text-muted-foreground">
              Archive “{template.name}”? It will be hidden from the active list.
              You can inspect and restore it from Archived. Its exercise
              references will remain.
            </p>
            {actionError && (
              <Alert variant="destructive">
                <AlertTitle>Could not archive template</AlertTitle>
                <AlertDescription>{actionError}</AlertDescription>
              </Alert>
            )}
            <div className="flex flex-wrap gap-3">
              <Button
                autoFocus
                variant="outline"
                disabled={busy}
                onClick={() => setAction(null)}
              >
                Cancel
              </Button>
              <Button disabled={busy} onClick={() => void mutate("archive")}>
                {busy ? "Archiving…" : "Confirm archive"}
              </Button>
            </div>
          </div>
        )}
      </dialog>
    </>
  );
}
