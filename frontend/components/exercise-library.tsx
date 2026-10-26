"use client";

import { useCallback, useEffect, useState, type FormEvent } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { toast } from "sonner";
import { ExerciseRow } from "@/components/exercise-row";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { muscleGroupLabel, type ExercisePage } from "@/lib/exercises";
import { MuscleGroupPicker } from "@/components/muscle-group-picker";

export function ExerciseLibrary() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const criteria = searchParams.toString();
  const appliedQuery = searchParams.get("q") ?? "";
  const appliedFilter = searchParams.get("muscle_group") ?? "";
  const [result, setResult] = useState<ExercisePage | null>(null);
  const [loadedCriteria, setLoadedCriteria] = useState<string | null>(null);
  const [revision, setRevision] = useState(0);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [groups, setGroups] = useState<string[]>([]);
  const [tags, setTags] = useState<string[]>([]);
  const [tagError, setTagError] = useState("");
  const [query, setQuery] = useState(appliedQuery);
  const [filter, setFilter] = useState(appliedFilter);
  const [error, setError] = useState("");
  const [listError, setListError] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [addedIDs, setAddedIDs] = useState<string[]>([]);

  const pageHref = useCallback(
    (page: number, query = appliedQuery, group = appliedFilter) => {
      const params = new URLSearchParams();
      if (query.trim()) params.set("q", query.trim());
      if (group) params.set("muscle_group", group);
      if (page !== 1) params.set("page", String(page));
      return `/exercises${params.size ? `?${params}` : ""}`;
    },
    [appliedQuery, appliedFilter],
  );

  useEffect(() => {
    setQuery(appliedQuery);
    setFilter(appliedFilter);
  }, [appliedQuery, appliedFilter]);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setListError("");
    const params = new URLSearchParams(criteria);
    params.set("paginated", "true");
    async function load() {
      try {
        const response = await fetch(`/api/exercises?${params}`, {
          cache: "no-store",
          signal: controller.signal,
        });
        const body = await response.json();
        if (!response.ok)
          throw new Error(
            body.error?.message ??
              "Could not load exercises. Please try again.",
          );
        if (controller.signal.aborted) return;
        const page = body as ExercisePage;
        setResult(page);
        setGroups(page.muscle_groups);
        setLoadedCriteria(criteria);
        const canonical = pageHref(page.page);
        if (canonical !== `/exercises${criteria ? `?${criteria}` : ""}`)
          router.replace(canonical, { scroll: false });
      } catch (error) {
        if (!controller.signal.aborted) {
          setListError(
            error instanceof Error
              ? error.message
              : "Could not load exercises. Please try again.",
          );
          setLoadedCriteria(criteria);
        }
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    }
    void load();
    return () => controller.abort();
  }, [criteria, revision, router, pageHref]);

  function refresh() {
    setLoading(true);
    setRevision((value) => value + 1);
  }

  async function addExercise(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (saving) return;
    setSaving(true);
    setError("");
    setTagError("");
    try {
      const response = await fetch("/api/exercises", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name, description, muscle_groups: tags }),
      });
      const body = await response.json();
      if (!response.ok) {
        setTagError(body.error?.fields?.muscle_groups ?? "");
        setError(
          body.error?.fields?.name ??
            body.error?.message ??
            "Could not add exercise. Please try again.",
        );
        return;
      }
      refresh();
      setAddedIDs((current) => [...current, body.id]);
      setName("");
      setDescription("");
      setTags([]);
      toast.success(`${body.name} added.`);
    } catch {
      setError("Could not add exercise. Please try again.");
    } finally {
      setSaving(false);
    }
  }

  const visible = result?.items ?? [];
  const waiting = loading || loadedCriteria !== criteria;

  return (
    <div className="grid gap-8 lg:grid-cols-[1fr_20rem]">
      <section aria-labelledby="exercise-list-heading" className="min-w-0">
        <h2 id="exercise-list-heading" className="mb-4 text-xl font-semibold">
          Exercise library
        </h2>
        <form
          className="mb-4 grid gap-3 sm:grid-cols-2"
          onSubmit={(event) => {
            event.preventDefault();
            router.push(pageHref(1, query, filter), { scroll: false });
          }}
        >
          <div>
            <label
              htmlFor="exercise-search"
              className="mb-2 block text-sm font-medium"
            >
              Search exercises
            </label>
            <input
              id="exercise-search"
              type="search"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              className="min-h-12 w-full rounded-lg border border-input bg-background px-3 text-base focus-visible:outline-2 focus-visible:outline-ring"
              placeholder="Search by name"
            />
          </div>
          <div>
            <label
              htmlFor="muscle-filter"
              className="mb-2 block text-sm font-medium"
            >
              Filter by muscle group
            </label>
            <select
              id="muscle-filter"
              value={filter}
              onChange={(event) => setFilter(event.target.value)}
              className="min-h-12 w-full rounded-lg border border-input bg-background px-3 text-base focus-visible:outline-2 focus-visible:outline-ring"
            >
              <option value="">All muscle groups</option>
              {groups.map((group) => (
                <option key={group} value={group}>
                  {muscleGroupLabel(group)}
                </option>
              ))}
            </select>
          </div>
          <Button type="submit" variant="outline">
            Apply search and filter
          </Button>
        </form>
        {waiting ? (
          <p role="status">Loading exercises…</p>
        ) : listError ? (
          <Alert variant="destructive">
            <AlertTitle>Could not load exercises</AlertTitle>
            <AlertDescription>{listError}</AlertDescription>
            <Button onClick={refresh} className="mt-3">
              Try again
            </Button>
          </Alert>
        ) : visible.length === 0 && !appliedQuery && !appliedFilter ? (
          <p className="rounded-2xl border border-dashed border-border p-6 text-muted-foreground">
            No exercises yet. Add your first exercise using the form.
          </p>
        ) : visible.length === 0 ? (
          <div className="rounded-2xl border border-dashed border-border p-6">
            <p className="text-muted-foreground">
              No exercises match your search and filter.
            </p>
            <Button
              variant="outline"
              className="mt-3"
              onClick={() => {
                router.push("/exercises", { scroll: false });
              }}
            >
              Clear search and filter
            </Button>
          </div>
        ) : (
          <ul className="space-y-4">
            {visible.map((exercise) => (
              <ExerciseRow
                key={exercise.id}
                exercise={exercise}
                animateEntrance={addedIDs.includes(exercise.id)}
                groups={groups}
                onUpdated={refresh}
                onDeleted={refresh}
              />
            ))}
          </ul>
        )}
        {!waiting && !listError && result && result.total_pages > 0 && (
          <nav aria-label="Exercise pages" className="mt-6">
            <p role="status" className="mb-3 text-sm text-muted-foreground">
              Page {result.page} of {result.total_pages}
            </p>
            <div className="flex flex-wrap gap-2">
              {Array.from(
                { length: result.total_pages },
                (_, index) => index + 1,
              ).map((page) => (
                <Button
                  key={page}
                  asChild
                  variant={page === result.page ? "default" : "outline"}
                >
                  <Link
                    href={pageHref(page)}
                    scroll={false}
                    aria-label={`Page ${page}`}
                    aria-current={page === result.page ? "page" : undefined}
                  >
                    {page}
                  </Link>
                </Button>
              ))}
            </div>
          </nav>
        )}
      </section>
      <section
        aria-labelledby="add-exercise-heading"
        className="self-start rounded-2xl border border-border bg-card p-5"
      >
        <h2 id="add-exercise-heading" className="text-xl font-semibold">
          Add an exercise
        </h2>
        <form
          onSubmit={addExercise}
          noValidate
          className="mt-5 flex flex-col gap-3"
        >
          <label htmlFor="exercise-name" className="text-sm font-medium">
            Exercise name
          </label>
          <input
            id="exercise-name"
            name="name"
            type="text"
            value={name}
            disabled={saving}
            onChange={(event) => setName(event.target.value)}
            aria-invalid={Boolean(error)}
            aria-describedby={
              error ? "exercise-help exercise-error" : "exercise-help"
            }
            className="min-h-12 w-full rounded-lg border border-input bg-background px-3 text-base focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
            placeholder="e.g. Overhead press"
          />
          <label htmlFor="exercise-description" className="text-sm font-medium">
            Description (optional)
          </label>
          <textarea
            id="exercise-description"
            value={description}
            disabled={saving}
            onChange={(event) => setDescription(event.target.value)}
            rows={3}
            className="min-h-24 w-full rounded-lg border border-input bg-background p-3 text-base focus-visible:outline-2 focus-visible:outline-ring"
          />
          <p id="exercise-help" className="text-sm text-muted-foreground">
            Choose a name you will recognize in your workouts.
          </p>
          <fieldset disabled={saving}>
            <MuscleGroupPicker
              groups={groups}
              selected={tags}
              onChange={setTags}
              error={tagError}
            />
          </fieldset>
          {error && (
            <Alert variant="destructive">
              <AlertTitle>Could not add exercise</AlertTitle>
              <AlertDescription id="exercise-error">{error}</AlertDescription>
            </Alert>
          )}
          <Button
            type="submit"
            disabled={saving || loading}
            className="mt-2 min-h-12 rounded-lg bg-primary px-4 py-3 font-semibold text-primary-foreground hover:bg-primary/90"
          >
            {saving ? "Saving…" : "Add exercise"}
          </Button>
        </form>
      </section>
    </div>
  );
}
