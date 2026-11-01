"use client";

import { useEffect, useState, type ReactNode } from "react";
import { usePathname } from "next/navigation";
import { Button } from "@/components/ui/button";

function leaveAccount() {
  try {
    const channel = new BroadcastChannel("crow-auth");
    channel.postMessage("signed-out");
    channel.close();
  } catch {}
  window.location.replace("/login");
}

export function Login() {
  const [csrf, setCSRF] = useState("");
  const [error, setError] = useState("");
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setError("");
    setCSRF("");
    if (new URLSearchParams(window.location.search).has("error"))
      setError("Google sign-in was not completed. Please try again.");
    async function initialize() {
      try {
        // Finish the identity check before creating anonymous CSRF state: an
        // unauthenticated check expires the old cookie and must not race it.
        const identity = await fetch("/api/auth/me", {
          cache: "no-store",
          signal: controller.signal,
        });
        if (identity.ok) {
          if (!controller.signal.aborted) window.location.replace("/");
          return;
        }
        if (identity.status !== 401) throw new Error("unavailable");
        const response = await fetch("/api/auth/csrf", {
          cache: "no-store",
          signal: controller.signal,
        });
        if (!response.ok) throw new Error("unavailable");
        const body = await response.json();
        if (!controller.signal.aborted) setCSRF(body.csrf);
      } catch {
        if (!controller.signal.aborted)
          setError("Could not start sign-in. Please try again.");
      }
    }
    void initialize();
    return () => controller.abort();
  }, [retry]);
  return (
    <div className="mt-6 space-y-4">
      {error && <p role="alert">{error}</p>}
      <form action="/api/auth/google/start" method="post">
        <input type="hidden" name="csrf" value={csrf} />
        <Button type="submit" disabled={!csrf}>
          Continue with Google
        </Button>
      </form>
      {!csrf && (
        <Button
          variant="outline"
          onClick={() => setRetry((value) => value + 1)}
        >
          Try again
        </Button>
      )}
    </div>
  );
}

export function AuthBoundary({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const publicPage = pathname === "/login";
  const [ready, setReady] = useState(false);
  const [name, setName] = useState("");
  const [error, setError] = useState("");
  const [retry, setRetry] = useState(0);
  const [csrf, setCSRF] = useState("");
  useEffect(() => {
    if (publicPage) return;
    const controller = new AbortController();
    let active = true;
    const original = window.fetch.bind(window);
    const requests = new Set<AbortController>();
    const channel = new BroadcastChannel("crow-auth");
    function clear() {
      active = false;
      setReady(false);
      for (const request of requests) request.abort();
      window.location.replace("/login");
    }
    channel.onmessage = clear;
    async function initialize() {
      setReady(false);
      setError("");
      try {
        const [response, tokens] = await Promise.all([
          original("/api/auth/me", {
            cache: "no-store",
            signal: controller.signal,
            headers:
              document.visibilityState === "visible"
                ? { "X-Crow-Activity": "foreground" }
                : undefined,
          }),
          original("/api/auth/csrf", {
            cache: "no-store",
            signal: controller.signal,
          }),
        ]);
        if (response.status === 401) {
          clear();
          return;
        }
        if (!response.ok || !tokens.ok) throw new Error("unavailable");
        const account = await response.json();
        const token = await tokens.json();
        if (!active) return;
        setName(account.name || account.email || "Signed in");
        setCSRF(token.csrf);
        // Install the authenticated transport before mounting data-fetching children.
        window.fetch = async (input, init) => {
          const url = new URL(
            input instanceof Request ? input.url : input.toString(),
            window.location.origin,
          );
          if (
            url.origin !== window.location.origin ||
            !url.pathname.startsWith("/api/")
          )
            return original(input, init);
          const request = new Request(
            input instanceof Request ? input : new URL(url),
            init,
          );
          const abort = new AbortController();
          requests.add(abort);
          const headers = new Headers(request.headers);
          if (!["GET", "HEAD"].includes(request.method))
            headers.set("X-CSRF-Token", token.csrf);
          if (document.visibilityState === "visible")
            headers.set("X-Crow-Activity", "foreground");
          try {
            const result = await original(
              new Request(request, {
                headers,
                signal: AbortSignal.any([request.signal, abort.signal]),
              }),
            );
            if (!active)
              throw new DOMException("Account changed", "AbortError");
            if (result.status === 401) {
              channel.postMessage("signed-out");
              clear();
              throw new Error("Sign in again");
            }
            return result;
          } finally {
            requests.delete(abort);
          }
        };
        setReady(true);
      } catch {
        if (active)
          setError("Could not verify your session. Please try again.");
      }
    }
    function validate() {
      if (document.visibilityState !== "visible") return;
      void original("/api/auth/me", {
        cache: "no-store",
        signal: controller.signal,
      })
        .then((r) => {
          if (r.status === 401) {
            channel.postMessage("signed-out");
            clear();
          } else if (!r.ok) {
            setReady(false);
            setError("Could not verify your session. Please try again.");
          }
        })
        .catch(() => {
          if (active) {
            setReady(false);
            setError("Could not verify your session. Please try again.");
          }
        });
    }
    function onPageShow(event: PageTransitionEvent) {
      if (event.persisted) {
        setReady(false);
        void initialize();
      }
    }
    document.addEventListener("visibilitychange", validate);
    window.addEventListener("pageshow", onPageShow);
    void initialize();
    return () => {
      active = false;
      controller.abort();
      requests.forEach((request) => request.abort());
      window.fetch = original;
      channel.close();
      document.removeEventListener("visibilitychange", validate);
      window.removeEventListener("pageshow", onPageShow);
    };
  }, [publicPage, retry]);
  if (publicPage) return children;
  if (!ready)
    return (
      <main className="mx-auto max-w-5xl px-6 py-8">
        <p role={error ? "alert" : "status"}>
          {error || "Checking your session…"}
        </p>
        {error && (
          <Button onClick={() => setRetry((value) => value + 1)}>
            Try again
          </Button>
        )}
      </main>
    );
  return (
    <>
      <div className="mx-auto flex max-w-5xl items-center justify-end gap-4 px-6 pt-4">
        <span className="min-w-0 break-words text-sm">{name}</span>
        <Button
          variant="outline"
          onClick={async () => {
            setError("");
            try {
              const response = await fetch("/api/auth/logout", {
                method: "POST",
                headers: { "X-CSRF-Token": csrf },
              });
              if (!response.ok) throw new Error();
              setReady(false);
              leaveAccount();
            } catch {
              setError("Could not sign out. Please try again.");
            }
          }}
        >
          Sign out
        </Button>
        {error && <p role="alert">{error}</p>}
      </div>
      {children}
    </>
  );
}
