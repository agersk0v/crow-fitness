"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Alert, AlertTitle, AlertDescription } from "@/components/ui/alert";
import type { Session } from "@/lib/sessions";

export function ActiveSession() {
  const [session, setSession] = useState<Session | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError("");
    async function load() {
      try {
        const response = await fetch("/api/sessions/active", {
          cache: "no-store",
          signal: controller.signal,
        });
        if (!response.ok)
          throw new Error(
            "Could not check your active session. Please try again.",
          );
        const body = await response.json();
        if (!controller.signal.aborted) setSession(body.session);
      } catch {
        if (!controller.signal.aborted)
          setError("Could not check your active session. Please try again.");
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    }
    void load();
    return () => controller.abort();
  }, [retry]);
  if (loading)
    return (
      <p role="status" className="text-sm text-muted-foreground">
        Checking active session…
      </p>
    );
  if (error)
    return (
      <Alert variant="destructive">
        <AlertTitle>Could not load session</AlertTitle>
        <AlertDescription>{error}</AlertDescription>
        <Button
          className="col-start-2 mt-3"
          onClick={() => setRetry(retry + 1)}
        >
          Try again
        </Button>
      </Alert>
    );
  if (!session) return null;
  return (
    <section
      className="rounded-2xl border border-border bg-card p-5"
      aria-label="Active workout"
    >
      <p className="text-sm text-muted-foreground">Active workout</p>
      <h2 className="mt-2 break-words text-2xl font-semibold">
        {session.name}
      </h2>
      <Link
        href={`/sessions/${session.id}`}
        className="mt-4 inline-block rounded-lg bg-primary px-5 py-3 font-semibold text-primary-foreground hover:bg-primary/90"
      >
        Continue session
      </Link>
    </section>
  );
}
