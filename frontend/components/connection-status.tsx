"use client";

import { useEffect, useState } from "react";

type Status = "checking" | "connected" | "unavailable";

async function checkEndpoint(
  path: string,
  signal: AbortSignal,
): Promise<Status> {
  try {
    const response = await fetch(path, { cache: "no-store", signal });
    if (!response.ok) return "unavailable";
    const result: unknown = await response.json();
    return typeof result === "object" &&
      result !== null &&
      "status" in result &&
      result.status === "ok"
      ? "connected"
      : "unavailable";
  } catch {
    return "unavailable";
  }
}

export function ConnectionStatus() {
  const [api, setApi] = useState<Status>("checking");
  const [database, setDatabase] = useState<Status>("checking");
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let active = true;
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), 6000);
    setApi("checking");
    setDatabase("checking");

    Promise.all([
      checkEndpoint("/api/health", controller.signal),
      checkEndpoint("/api/ready", controller.signal),
    ]).then(([apiStatus, databaseStatus]) => {
      clearTimeout(timeout);
      if (active) {
        setApi(apiStatus);
        setDatabase(databaseStatus);
      }
    });

    return () => {
      active = false;
      clearTimeout(timeout);
      controller.abort();
    };
  }, [attempt]);

  const labels = {
    checking: "Checking…",
    connected: "Connected",
    unavailable: "Unavailable",
  };
  const checking = api === "checking" || database === "checking";

  return (
    <section
      className="max-w-xl rounded-2xl border border-border bg-card p-6"
      aria-label="Local setup status"
    >
      <div className="mb-5 flex items-center justify-between gap-4">
        <h2 className="text-lg font-semibold">Connection check</h2>
        <button
          className="rounded-lg border border-border px-3 py-2 text-sm transition hover:bg-accent disabled:opacity-50"
          onClick={() => setAttempt((value) => value + 1)}
          disabled={checking}
        >
          Check again
        </button>
      </div>
      <dl className="space-y-3 text-sm" aria-live="polite">
        {[
          { label: "Workout API", status: api },
          { label: "Database", status: database },
        ].map(({ label, status }) => (
          <div key={label} className="flex justify-between gap-4">
            <dt className="text-muted-foreground">{label}</dt>
            <dd
              className={
                status === "connected"
                  ? "text-highlight"
                  : status === "unavailable"
                    ? "text-warning"
                    : "text-muted-foreground"
              }
            >
              {labels[status]}
            </dd>
          </div>
        ))}
      </dl>
      {!checking && (api !== "connected" || database !== "connected") && (
        <p className="mt-4 text-sm leading-relaxed text-warning">
          Part of the local setup could not be reached. Check the running
          services, then try again.
        </p>
      )}
    </section>
  );
}
