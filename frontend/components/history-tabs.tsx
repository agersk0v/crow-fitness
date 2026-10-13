"use client";
import { useRef, type KeyboardEvent } from "react";
import { useRouter } from "next/navigation";
import { Button } from "@/components/ui/button";
import { CompletedSessions } from "@/components/completed-sessions";
import { CompletedSets } from "@/components/completed-sets";

const tabs = ["sessions", "sets"] as const;
export function HistoryTabs({ selected }: { selected: "sessions" | "sets" }) {
  const router = useRouter();
  const refs = useRef<(HTMLButtonElement | null)[]>([]);
  function select(tab: "sessions" | "sets") {
    router.push(`/history?tab=${tab}`, { scroll: false });
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
      <div
        role="tabpanel"
        id="history-sessions-panel"
        aria-labelledby="history-sessions-tab"
        hidden={selected !== "sessions"}
        tabIndex={0}
      >
        {selected === "sessions" && <CompletedSessions />}
      </div>
      <div
        role="tabpanel"
        id="history-sets-panel"
        aria-labelledby="history-sets-tab"
        hidden={selected !== "sets"}
        tabIndex={0}
      >
        {selected === "sets" && <CompletedSets />}
      </div>
    </div>
  );
}
