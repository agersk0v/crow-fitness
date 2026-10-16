"use client";
import { useEffect, useState, type FormEvent } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import {
  historyCriteria,
  historyFilterError,
  historyQuery,
} from "@/lib/history-filters";

export function HistoryFilters({
  query,
  selected,
}: {
  query: string;
  selected: "sessions" | "sets";
}) {
  const router = useRouter();
  const kind = selected === "sessions" ? "template" : "exercise";
  const label = selected === "sessions" ? "Template" : "Exercise";
  const [draft, setDraft] = useState(() =>
    historyCriteria(new URLSearchParams(query)),
  );
  const [error, setError] = useState("");
  const [options, setOptions] = useState<{ id: string; name: string }[]>([]);
  const [loading, setLoading] = useState(true);
  const [failed, setFailed] = useState(false);
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setFailed(false);
    async function load() {
      try {
        const response = await fetch(`/api/history/${kind}s`, {
          cache: "no-store",
          signal: controller.signal,
        });
        if (!response.ok) throw new Error();
        const body = await response.json();
        if (!controller.signal.aborted) setOptions(body.items);
      } catch {
        if (!controller.signal.aborted) setFailed(true);
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    }
    void load();
    return () => controller.abort();
  }, [retry, kind]);
  function apply(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const params = historyQuery(draft, selected);
    const message = historyFilterError(params);
    if (message) {
      setError(message);
      return;
    }
    setError("");
    router.push(`/history${params.size ? `?${params}` : ""}`, {
      scroll: false,
    });
  }
  const inputClass =
    "mt-2 block min-h-11 w-full rounded-lg border border-border bg-background px-3 py-2 text-foreground focus-visible:outline-2 focus-visible:outline-ring";
  return (
    <form
      onSubmit={apply}
      className="space-y-4 rounded-xl border border-border p-4"
      aria-label="History search and filters"
    >
      {selected === "sessions" && draft.session && (
        <div className="flex flex-wrap items-center justify-between gap-3">
          <p className="text-sm font-medium">
            Showing workout #{draft.session}
          </p>
          <Button
            type="button"
            variant="outline"
            onClick={() => {
              const params = new URLSearchParams(query);
              params.delete("session");
              params.set("tab", "sessions");
              router.push(`/history?${params}`, { scroll: false });
            }}
          >
            Clear workout filter
          </Button>
        </div>
      )}
      <div className="grid gap-4 sm:grid-cols-2">
        <label className="text-sm font-medium">
          Search names
          <input
            type="search"
            value={draft.q}
            onChange={(event) => setDraft({ ...draft, q: event.target.value })}
            className={inputClass}
            placeholder="Workout or exercise name"
          />
        </label>
        <div className="text-sm font-medium">
          <label htmlFor="history-identity-filter">{label}</label>
          <select
            id="history-identity-filter"
            value={draft[kind]}
            onChange={(event) =>
              setDraft({ ...draft, [kind]: event.target.value })
            }
            className={inputClass}
            disabled={loading || failed}
          >
            <option value="">All {kind}s</option>
            {draft[kind] &&
              !options.some((option) => option.id === draft[kind]) && (
                <option value={draft[kind]}>
                  {label} #{draft[kind]}
                </option>
              )}
            {options.map((option) => (
              <option key={option.id} value={option.id}>
                {option.name} (#{option.id})
              </option>
            ))}
          </select>
        </div>
        <label className="text-sm font-medium">
          From date
          <input
            type="date"
            value={draft.from}
            onChange={(event) =>
              setDraft({ ...draft, from: event.target.value })
            }
            className={inputClass}
          />
        </label>
        <label className="text-sm font-medium">
          To date
          <input
            type="date"
            value={draft.to}
            onChange={(event) => setDraft({ ...draft, to: event.target.value })}
            className={inputClass}
          />
        </label>
      </div>
      <p className="text-sm text-muted-foreground">
        Completion dates use your local timezone. Search matches saved workout
        and exercise names.
      </p>
      {loading && (
        <p role="status" className="text-sm text-muted-foreground">
          Loading {kind} filters…
        </p>
      )}
      {failed && (
        <div className="space-y-2">
          <p role="alert">Could not load {kind} filters.</p>
          <Button
            type="button"
            variant="outline"
            onClick={() => setRetry(retry + 1)}
          >
            Retry {kind} filters
          </Button>
        </div>
      )}
      {error && (
        <p role="alert" className="text-destructive">
          {error}
        </p>
      )}
      <div className="flex flex-wrap gap-3">
        <Button type="submit">Apply search and filters</Button>
        <Button
          type="button"
          variant="outline"
          onClick={() => {
            setDraft({
              q: "",
              exercise: "",
              template: "",
              session: "",
              from: "",
              to: "",
            });
            setError("");
            router.push(
              selected === "sessions" ? "/history?tab=sessions" : "/history",
              { scroll: false },
            );
          }}
        >
          Clear filters
        </Button>
      </div>
    </form>
  );
}
