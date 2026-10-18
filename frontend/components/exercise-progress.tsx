"use client";

import { useEffect, useId, useState } from "react";
import Link from "next/link";
import { CartesianGrid, Line, LineChart, XAxis, YAxis } from "recharts";
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from "@/components/ui/chart";
import { Button } from "@/components/ui/button";
import {
  partialRepsLabel,
  techniqueLabel,
  type Technique,
} from "@/lib/sessions";

type ProgressPoint = {
  session_id: string;
  session_name: string;
  completed_at: string;
  exercise_name: string;
  exercise_position: number;
  set_number: number;
  set_id: string;
  weight: string;
  reps: string;
  rir: string | null;
  technique: Technique;
  partial_reps: number | null;
};
type Datum = ProgressPoint & { value: number };
const config = { value: { label: "Full reps", color: "var(--highlight)" } };

export function ExerciseProgress({ query }: { query: string }) {
  const id = useId();
  const [points, setPoints] = useState<ProgressPoint[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [unavailable, setUnavailable] = useState(false);
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError(false);
    setUnavailable(false);
    async function load() {
      try {
        const response = await fetch(`/api/history/progress?${query}`, {
          cache: "no-store",
          signal: controller.signal,
        });
        if (!response.ok) throw new Error();
        const body = await response.json();
        if (!Array.isArray(body.items)) throw new Error();
        if (!controller.signal.aborted) {
          setUnavailable(body.metric !== "max_full_reps");
          setPoints(body.items);
        }
      } catch {
        if (!controller.signal.aborted) setError(true);
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    }
    void load();
    return () => controller.abort();
  }, [query, retry]);
  const data: Datum[] = points.map((point) => ({
    ...point,
    value: Number(point.reps),
  }));
  const dates = new Map(
    data.map((point) => [
      point.session_id,
      new Date(point.completed_at).toLocaleDateString(undefined, {
        month: "short",
        day: "numeric",
      }),
    ]),
  );
  return (
    <section
      aria-labelledby={`${id}-title`}
      className="mb-6 min-w-0 rounded-2xl border border-border bg-card p-4 sm:p-5"
    >
      <h2 id={`${id}-title`} className="text-lg font-semibold">
        Most full reps in one set
      </h2>
      <p
        id={`${id}-description`}
        className="mt-2 text-sm text-muted-foreground"
      >
        One set per completed workout, using the applied filters. Weight,
        assistance, and partial reps do not change this value. Higher reps at a
        different load do not necessarily mean greater strength.
      </p>
      {loading ? (
        <p role="status" className="mt-4 text-sm text-muted-foreground">
          Loading progress…
        </p>
      ) : error ? (
        <div className="mt-4 space-y-3">
          <p role="alert" className="text-sm text-destructive">
            Could not load progress. Recorded sets are available separately
            below.
          </p>
          <Button
            variant="outline"
            onClick={() => setRetry((value) => value + 1)}
          >
            Retry progress
          </Button>
        </div>
      ) : unavailable ? (
        <p role="status" className="mt-4 text-sm text-muted-foreground">
          This progress metric is unavailable. You can still browse recorded
          sets below.
        </p>
      ) : !points.length ? (
        <p className="mt-4 text-sm text-muted-foreground">
          No completed sets match this exercise and these filters. Complete a
          workout with saved sets or adjust the filters.
        </p>
      ) : (
        <>
          <p className="mt-3 break-words text-sm font-medium">
            {points.at(-1)!.exercise_name} · {points.length}{" "}
            {points.length === 1
              ? "workout — one point, no trend yet"
              : "workouts"}
          </p>
          <ChartContainer
            config={config}
            className="mt-4 h-52 w-full aspect-auto [&_.recharts-surface:focus-visible]:rounded-lg [&_.recharts-surface:focus-visible]:outline-2 [&_.recharts-surface:focus-visible]:outline-ring"
            aria-describedby={`${id}-description`}
          >
            <LineChart
              accessibilityLayer
              data={data}
              margin={{ top: 12, right: 12, bottom: 4, left: 0 }}
              aria-label="Full-rep records by completed workout. Use left and right arrow keys to inspect points, or open View chart data below."
            >
              <CartesianGrid vertical={false} />
              <XAxis
                dataKey="session_id"
                tickFormatter={(value) => dates.get(String(value)) ?? ""}
                tickLine={false}
                axisLine={false}
                minTickGap={28}
                tickMargin={8}
              />
              <YAxis
                allowDecimals={false}
                domain={[0, "auto"]}
                tickLine={false}
                axisLine={false}
                width={36}
                tickFormatter={(value) =>
                  Number(value).toLocaleString(undefined, {
                    notation: "compact",
                  })
                }
              />
              <ChartTooltip
                isAnimationActive={false}
                content={
                  <ChartTooltipContent
                    hideLabel
                    hideIndicator
                    className="max-w-[min(17rem,calc(100vw-4rem))]"
                    formatter={(_value, _name, item) => (
                      <PointDetails point={item.payload as Datum} />
                    )}
                  />
                }
              />
              <Line
                type="linear"
                dataKey="value"
                stroke="var(--color-value)"
                strokeWidth={2}
                dot={{ r: 4, fill: "var(--color-value)" }}
                activeDot={{ r: 6 }}
                isAnimationActive={false}
              />
            </LineChart>
          </ChartContainer>
          <p className="mt-2 text-xs text-muted-foreground">
            Completed workouts, oldest first. Tap a point or focus the chart and
            use arrow keys. Equal-date workouts remain separate.
          </p>
          <details className="mt-4">
            <summary className="cursor-pointer rounded text-sm font-medium focus-visible:outline-2 focus-visible:outline-ring">
              View chart data ({points.length}{" "}
              {points.length === 1 ? "workout" : "workouts"})
            </summary>
            <ol aria-label="Progress source sets" className="mt-3 space-y-3">
              {points.map((point) => (
                <li
                  key={point.session_id}
                  className="rounded-lg bg-muted/30 p-3 text-sm"
                >
                  <Link
                    href={`/history?tab=sessions&session=${point.session_id}`}
                    className="break-words font-medium text-highlight underline"
                  >
                    {point.session_name}
                  </Link>
                  <PointDetails point={point} showName={false} />
                </li>
              ))}
            </ol>
          </details>
        </>
      )}
    </section>
  );
}

function PointDetails({
  point,
  showName = true,
}: {
  point: ProgressPoint;
  showName?: boolean;
}) {
  return (
    <div className="min-w-0 space-y-1 break-words text-xs">
      <p className="font-medium">
        {point.reps} full reps · {point.weight} kg
      </p>
      {showName && <p>{point.session_name}</p>}
      <p>{new Date(point.completed_at).toLocaleString()}</p>
      <p>
        Exercise {point.exercise_position}, set {point.set_number} ·{" "}
        {techniqueLabel(point.technique)}
      </p>
      {point.rir !== null && <p>RIR {point.rir}</p>}
      {!!point.partial_reps && (
        <p>{partialRepsLabel(point.partial_reps)} (context only)</p>
      )}
    </div>
  );
}
