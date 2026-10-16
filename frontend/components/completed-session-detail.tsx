"use client";
import { CompletedSessionExercises } from "@/components/completed-session-exercises";
import { useEffect, useState } from "react";
import { Button } from "@/components/ui/button";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import type { Session } from "@/lib/sessions";

export function CompletedSessionDetail({
  id,
  inline = false,
}: {
  id: string;
  inline?: boolean;
}) {
  const Title = inline ? "h3" : "h1";
  const [session, setSession] = useState<Session | null>(null);
  const [loading, setLoading] = useState(true);
  const [missing, setMissing] = useState(false);
  const [error, setError] = useState("");
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setMissing(false);
    setError("");
    async function load() {
      try {
        const response = await fetch(
          `/api/history/sessions/${encodeURIComponent(id)}`,
          { cache: "no-store", signal: controller.signal },
        );
        if (response.status === 404) {
          if (!controller.signal.aborted) setMissing(true);
          return;
        }
        if (!response.ok) throw new Error();
        const body: Session = await response.json();
        if (!controller.signal.aborted) setSession(body);
      } catch {
        if (!controller.signal.aborted)
          setError("Could not load this completed session. Please try again.");
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    }
    void load();
    return () => controller.abort();
  }, [id, retry]);
  if (loading) return <p role="status">Loading completed session…</p>;
  if (missing)
    return (
      <div className="space-y-3">
        <Title className="text-xl font-semibold">
          Completed session not found
        </Title>
        <p className="text-muted-foreground">
          This completed workout could not be found.
        </p>
      </div>
    );
  if (error)
    return (
      <Alert variant="destructive">
        <AlertTitle>Could not open completed session</AlertTitle>
        <AlertDescription>{error}</AlertDescription>
        <Button
          className="col-start-2 mt-3"
          onClick={() => setRetry(retry + 1)}
        >
          Try again
        </Button>
      </Alert>
    );
  if (!session || !session.completed_at) return null;
  return (
    <>
      {!inline && (
        <>
          <p className="text-sm font-medium text-highlight">
            Completed workout
          </p>
          <h1 className="mt-2 break-words text-4xl font-semibold">
            {session.name}
          </h1>
          <time
            className="mt-3 block text-sm text-muted-foreground"
            dateTime={session.completed_at}
          >
            {new Date(session.completed_at).toLocaleString()}
          </time>
        </>
      )}
      {!inline && (
        <p className="my-6 text-muted-foreground">
          This is your saved workout. Template targets are separate from the
          sets you actually recorded.
        </p>
      )}
      <CompletedSessionExercises session={session} inline={inline} />
    </>
  );
}
