"use client";
import { useRef, type KeyboardEvent } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Button } from "@/components/ui/button";
import { CompletedSessions } from "@/components/completed-sessions";
import { CompletedSets } from "@/components/completed-sets";

import { HistoryFilters } from "@/components/history-filters";
import {
  historyCriteria,
  historyFilterError,
  historyQuery,
} from "@/lib/history-filters";

const tabs = ["sets", "sessions"] as const;
export function HistoryTabs({ selected }: { selected: "sessions" | "sets" }) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const query = searchParams.toString();
  const params = new URLSearchParams(query);
  const criteria = historyCriteria(params);
  const validation = historyFilterError(params);
  const filters = historyQuery(criteria);
  filters.delete(selected === "sessions" ? "exercise" : "template");
  if (selected === "sets") filters.delete("session");
  if (criteria.from || criteria.to)
    filters.set("tz", Intl.DateTimeFormat().resolvedOptions().timeZone);
  const apiQuery = filters.toString();
  const filtered = [...filters.values()].some((value) => value.trim() !== "");
  const refs = useRef<(HTMLButtonElement | null)[]>([]);
  function select(tab: "sessions" | "sets") {
    const next = new URLSearchParams(query);
    if (tab === "sessions") next.set("tab", "sessions");
    else next.delete("tab");
    router.push(`/history${next.size ? `?${next}` : ""}`, { scroll: false });
  }
  function move(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    let next: number;
    switch (event.key) {
      case "ArrowRight":
        next = (index + 1) % tabs.length;
        break;
      case "ArrowLeft":
        next = (index + tabs.length - 1) % tabs.length;
        break;
      case "Home":
        next = 0;
        break;
      case "End":
        next = tabs.length - 1;
        break;
      default:
        return;
    }
    event.preventDefault();
    refs.current[next]?.focus();
    select(tabs[next]);
  }
  return (
    <div className="space-y-6">
      <div
        role="tablist"
        aria-label="Workout history views"
        className="flex gap-3"
      >
        {tabs.map((tab, index) => (
          <Button
            key={tab}
            ref={(node) => {
              refs.current[index] = node;
            }}
            type="button"
            role="tab"
            id={`history-${tab}-tab`}
            aria-controls={`history-${tab}-panel`}
            aria-selected={selected === tab}
            tabIndex={selected === tab ? 0 : -1}
            variant={selected === tab ? "default" : "outline"}
            onClick={() => select(tab)}
            onKeyDown={(event) => move(event, index)}
          >
            {tab === "sessions" ? "Sessions" : "Sets"}
          </Button>
        ))}
      </div>
      <HistoryFilters key={query} query={query} selected={selected} />
      {validation && (
        <p role="alert" className="text-destructive">
          {validation}
        </p>
      )}
      <div
        role="tabpanel"
        id="history-sets-panel"
        aria-labelledby="history-sets-tab"
        hidden={selected !== "sets"}
        tabIndex={0}
      >
        {selected === "sets" && !validation && (
          <CompletedSets key={apiQuery} query={apiQuery} filtered={filtered} />
        )}
      </div>
      <div
        role="tabpanel"
        id="history-sessions-panel"
        aria-labelledby="history-sessions-tab"
        hidden={selected !== "sessions"}
        tabIndex={0}
      >
        {selected === "sessions" && !validation && (
          <CompletedSessions
            key={apiQuery}
            query={apiQuery}
            filtered={filtered}
          />
        )}
      </div>
    </div>
  );
}
