"use client";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import { TemplateCard } from "@/components/template-card";
import { TemplateForm } from "@/components/template-form";
import type { Exercise } from "@/lib/exercises";
import { useDraftNavigation } from "@/hooks/use-draft-navigation";
import type { TemplateEditorState } from "@/components/template-detail";
import type { TemplateSummary } from "@/lib/templates";

export function TemplateLibrary() {
  const [editorStates, setEditorStates] = useState<
    Record<string, TemplateEditorState>
  >({});
  const [starting, setStarting] = useState(false);
  const changeEditorState = useCallback(
    (id: string, state: TemplateEditorState | null) => {
      setEditorStates((current) => {
        if (
          state &&
          current[id]?.dirty === state.dirty &&
          current[id]?.busy === state.busy
        )
          return current;
        const next = { ...current };
        if (state) next[id] = state;
        else delete next[id];
        return next;
      });
    },
    [],
  );
  const canLeave = useDraftNavigation(
    Object.values(editorStates).some((state) => state.dirty),
    starting || Object.values(editorStates).some((state) => state.busy),
  );
  const [templates, setTemplates] = useState<TemplateSummary[]>([]);
  const [exercises, setExercises] = useState<Exercise[]>([]);
  const [archived, setArchived] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    async function load() {
      try {
        const [list, library] = await Promise.all([
          fetch(`/api/templates?archived=${archived}`, {
            cache: "no-store",
            signal: controller.signal,
          }),
          fetch("/api/exercises", {
            cache: "no-store",
            signal: controller.signal,
          }),
        ]);
        if (!list.ok || !library.ok) throw new Error("load failed");
        const [listBody, libraryBody] = await Promise.all([
          list.json(),
          library.json(),
        ]);
        if (!controller.signal.aborted) {
          setTemplates(listBody.items);
          setExercises(libraryBody.items);
        }
      } catch {
        if (!controller.signal.aborted)
          setError("Could not load templates and exercises. Please try again.");
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    }
    void load();
    return () => controller.abort();
  }, [archived, retry]);
  return (
    <div className="space-y-8">
      <section aria-labelledby="saved-templates">
        <h2 id="saved-templates" className="mb-4 text-xl font-semibold">
          {archived ? "Archived templates" : "Saved templates"}
        </h2>
        <div
          className="mb-4 flex gap-3"
          role="group"
          aria-label="Template list view"
        >
          <Button
            variant={archived ? "outline" : "default"}
            aria-pressed={!archived}
            onClick={() => {
              if (archived && canLeave()) setArchived(false);
            }}
          >
            Active
          </Button>
          <Button
            variant={archived ? "default" : "outline"}
            aria-pressed={archived}
            onClick={() => {
              if (!archived && canLeave()) setArchived(true);
            }}
          >
            Archived
          </Button>
        </div>
        {loading ? (
          <p role="status">Loading templates…</p>
        ) : error ? (
          <Alert variant="destructive">
            <AlertTitle>Could not load templates</AlertTitle>
            <AlertDescription>{error}</AlertDescription>
            <Button className="mt-3" onClick={() => setRetry(retry + 1)}>
              Try again
            </Button>
          </Alert>
        ) : templates.length === 0 ? (
          <p className="rounded-2xl border border-dashed border-border p-6 text-muted-foreground">
            {archived
              ? "No archived templates. Archived routines will appear here for inspection and restoration."
              : "No templates yet. Create your first routine below."}
          </p>
        ) : (
          <ul className="space-y-4">
            {templates.map((template) => (
              <TemplateCard
                key={template.id}
                template={template}
                canLeave={canLeave}
                starting={starting}
                onStartingChange={setStarting}
                onEditorStateChange={changeEditorState}
                onSaved={(saved) =>
                  setTemplates((current) =>
                    current.map((item) =>
                      item.id === saved.id ? saved : item,
                    ),
                  )
                }
                onCopied={(copy) =>
                  setTemplates((current) => [...current, copy])
                }
                onRestored={(saved) =>
                  setTemplates((current) =>
                    current.filter((item) => item.id !== saved.id),
                  )
                }
                onArchived={(id) =>
                  setTemplates((current) =>
                    current.filter((item) => item.id !== id),
                  )
                }
              />
            ))}
          </ul>
        )}
      </section>
      {!archived && (
        <section
          aria-labelledby="create-template"
          className="rounded-2xl border border-border bg-card p-5 sm:p-6"
        >
          <h2 id="create-template" className="text-xl font-semibold">
            Create a template
          </h2>
          <p className="mt-2 text-sm text-muted-foreground">
            Set counts cover working sets only. Optional rep and RIR targets
            apply to every working set.
          </p>
          {!loading && !error && exercises.length === 0 && (
            <p className="mt-4 text-muted-foreground">
              <Link href="/exercises" className="text-highlight underline">
                Add an exercise
              </Link>{" "}
              to your library before creating a template.
            </p>
          )}
          <TemplateForm
            exercises={exercises}
            disabled={starting || loading || Boolean(error)}
            onSaved={(template) =>
              setTemplates((current) => [...current, template])
            }
          />
        </section>
      )}
    </div>
  );
}
