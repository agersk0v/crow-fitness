"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { usePathname } from "next/navigation";
import { Button } from "@/components/ui/button";

function leaveAccount() {
  try {
    const channel = new BroadcastChannel("crow-auth");
    channel.postMessage("signed-out");
    channel.close();
  } catch {}
  window.location.replace("/");
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

type AuthState = {
  revalidating: boolean;
  status: "loading" | "signed-out" | "signed-in" | "error";
  activeSessionId: string | null;
  error: string;
  openLogin: () => void;
  signOut: () => Promise<void>;
  retry: () => void;
};
const AuthContext = createContext<AuthState>({
  revalidating: false,
  status: "loading",
  activeSessionId: null,
  error: "",
  openLogin: () => {},
  signOut: async () => {},
  retry: () => {},
});
export function useAuth() {
  return useContext(AuthContext);
}

// Shared identity/transport state surrounds both navigation and private content.
// Public Home remains rendered even while identity is unavailable.
export function AuthBoundary({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const [revalidating, setRevalidating] = useState(false);
  const [status, setStatus] = useState<AuthState["status"]>("loading");
  const [activeSessionId, setActiveSessionId] = useState<string | null>(null);
  const [error, setError] = useState("");
  const [retry, setRetry] = useState(0);
  const [csrf, setCSRF] = useState("");
  const [loginOpen, setLoginOpen] = useState(false);
  const dialog = useRef<HTMLDialogElement>(null);
  const logoutPending = useRef(false);
  const openLogin = useCallback(() => setLoginOpen(true), []);
  useEffect(() => {
    if (loginOpen) dialog.current?.showModal();
    else dialog.current?.close();
  }, [loginOpen]);
  useEffect(() => {
    window.dispatchEvent(new Event("crow-active-session"));
  }, [pathname]);
  useEffect(() => {
    const controller = new AbortController();
    let active = true;
    let accountID = "";
    let validation = 0;
    let checking = false;
    let sessionRequest: AbortController | null = null;
    const original = window.fetch.bind(window);
    const requests = new Set<AbortController>();
    const channel = new BroadcastChannel("crow-auth");
    function clear() {
      active = false;
      setStatus("signed-out");
      setActiveSessionId(null);
      sessionRequest?.abort();
      requests.forEach((r) => r.abort());
      window.location.replace("/");
    }
    channel.onmessage = clear;
    async function refreshActive() {
      if (!active || !accountID || checking) return;
      sessionRequest?.abort();
      const request = new AbortController();
      sessionRequest = request;
      try {
        const response = await original("/api/sessions/active", {
          cache: "no-store",
          signal: AbortSignal.any([controller.signal, request.signal]),
        });
        if (response.status === 401) {
          clear();
          return;
        }
        if (response.headers.get("X-Crow-Account") !== accountID) {
          clear();
          return;
        }
        if (!response.ok) throw new Error();
        const body = await response.json();
        if (active && !request.signal.aborted)
          setActiveSessionId(body.session?.id ?? null);
      } catch {
        if (active && !request.signal.aborted) setActiveSessionId(null);
      }
    }
    async function initialize() {
      setStatus("loading");
      setRevalidating(false);
      setError("");
      setActiveSessionId(null);
      try {
        // A 401 expires an old cookie; finish it before creating CSRF state.
        const response = await original("/api/auth/me", {
          cache: "no-store",
          signal: controller.signal,
          headers:
            document.visibilityState === "visible"
              ? { "X-Crow-Activity": "foreground" }
              : undefined,
        });
        if (!active) return;
        if (response.status === 401) {
          setStatus("signed-out");
          return;
        }
        if (!response.ok) throw new Error();
        const account = await response.json();
        const tokens = await original("/api/auth/csrf", {
          cache: "no-store",
          signal: controller.signal,
        });
        if (!tokens.ok) throw new Error();
        const token = await tokens.json();
        if (!active) return;
        accountID = String(account.id);
        setCSRF(token.csrf);
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
            input instanceof Request ? input : url,
            init,
          );
          if (checking || !active)
            throw new DOMException("Checking account", "AbortError");
          const generation = validation;
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
            if (!active || generation !== validation)
              throw new DOMException("Account changed", "AbortError");
            if (
              result.headers.has("X-Crow-Account") &&
              result.headers.get("X-Crow-Account") !== accountID
            ) {
              clear();
              throw new DOMException("Account changed", "AbortError");
            }
            if (result.status === 401) {
              channel.postMessage("signed-out");
              clear();
              throw new Error("Sign in again");
            }
            if (
              result.ok &&
              request.method === "POST" &&
              /^\/api\/sessions(?:$|\/[1-9][0-9]*\/(?:complete|cancel)$)/.test(
                url.pathname,
              )
            )
              void refreshActive();
            return result;
          } finally {
            requests.delete(abort);
          }
        };
        setStatus("signed-in");
        void refreshActive();
      } catch {
        if (active) {
          setStatus("error");
          setError("Could not verify your session. Please try again.");
        }
      }
    }
    function validate() {
      if (
        document.visibilityState !== "visible" ||
        !accountID ||
        checking ||
        !active
      )
        return;
      checking = true;
      validation++;
      setRevalidating(true);
      setActiveSessionId(null);
      sessionRequest?.abort();
      requests.forEach((request) => request.abort());
      void original("/api/auth/me", {
        cache: "no-store",
        signal: controller.signal,
      })
        .then(async (response) => {
          if (!active) return;
          if (response.status === 401) {
            if (accountID) clear();
            return;
          }
          if (!response.ok) throw new Error();
          const account = await response.json();
          if (!active) return;
          if (String(account.id) !== accountID) {
            window.location.reload();
            return;
          }
          checking = false;
          setRevalidating(false);
          void refreshActive();
        })
        .catch(() => {
          if (active) {
            setStatus("error");
            setActiveSessionId(null);
            setError("Could not verify your session. Please try again.");
          }
        });
    }
    function onPageShow(event: PageTransitionEvent) {
      if (event.persisted) {
        setStatus("loading");
        window.location.reload();
      }
    }
    document.addEventListener("visibilitychange", validate);
    window.addEventListener("pageshow", onPageShow);
    window.addEventListener("focus", validate);
    window.addEventListener("crow-active-session", refreshActive);
    void initialize();
    return () => {
      active = false;
      controller.abort();
      sessionRequest?.abort();
      requests.forEach((request) => request.abort());
      window.fetch = original;
      channel.close();
      document.removeEventListener("visibilitychange", validate);
      window.removeEventListener("pageshow", onPageShow);
      window.removeEventListener("focus", validate);
      window.removeEventListener("crow-active-session", refreshActive);
    };
  }, [retry]);
  async function signOut() {
    if (logoutPending.current) return;
    logoutPending.current = true;
    setError("");
    try {
      const response = await fetch("/api/auth/logout", {
        method: "POST",
        headers: { "X-CSRF-Token": csrf },
      });
      if (!response.ok) throw new Error();
      setStatus("signed-out");
      setActiveSessionId(null);
      leaveAccount();
    } catch {
      setError("Could not sign out. Please try again.");
    } finally {
      logoutPending.current = false;
    }
  }
  return (
    <AuthContext.Provider
      value={{
        status,
        revalidating,
        activeSessionId,
        error,
        openLogin,
        signOut,
        retry: () => setRetry((value) => value + 1),
      }}
    >
      {children}
      <dialog
        ref={dialog}
        aria-labelledby="login-dialog-title"
        onCancel={() => setLoginOpen(false)}
        onClose={() => setLoginOpen(false)}
        className="m-auto max-h-[90dvh] w-[calc(100%-2rem)] max-w-md overflow-y-auto rounded-2xl border border-border bg-card p-6 text-foreground backdrop:bg-black/60"
      >
        <div className="flex items-center justify-between gap-4">
          <h2 id="login-dialog-title" className="text-xl font-semibold">
            Log in to Crow Fitness
          </h2>
          <Button variant="outline" onClick={() => setLoginOpen(false)}>
            Close
          </Button>
        </div>
        <p className="mt-3 text-sm text-muted-foreground">
          Choose how you want to continue.
        </p>
        {loginOpen &&
          (status === "loading" ? (
            <p role="status" className="mt-4">
              Checking your session…
            </p>
          ) : (
            <Login />
          ))}
      </dialog>
    </AuthContext.Provider>
  );
}
export function AuthContent({ children }: { children: ReactNode }) {
  const pathname = usePathname();
  const auth = useAuth();
  if (pathname === "/" || pathname === "/login") return children;
  if (auth.status === "signed-in")
    return (
      <>
        {auth.revalidating && (
          <p role="status" className="p-6">
            Checking your session…
          </p>
        )}
        <div hidden={auth.revalidating} inert={auth.revalidating}>
          {children}
        </div>
      </>
    );
  return (
    <main className="mx-auto max-w-5xl px-6 py-8">
      <p role={auth.error ? "alert" : "status"}>
        {auth.error ||
          (auth.status === "loading"
            ? "Checking your session…"
            : "Log in to view your workouts.")}
      </p>
      {auth.error ? (
        <Button onClick={auth.retry}>Try again</Button>
      ) : auth.status === "signed-out" ? (
        <Button onClick={auth.openLogin}>Log in</Button>
      ) : null}
    </main>
  );
}
export function AuthControls() {
  const auth = useAuth();
  return (
    <div className="flex flex-wrap items-center gap-2">
      {auth.status === "signed-in" ? (
        <Button variant="outline" onClick={() => void auth.signOut()}>
          Log out
        </Button>
      ) : (
        <Button
          variant="outline"
          disabled={auth.status === "loading"}
          onClick={auth.openLogin}
        >
          Log in
        </Button>
      )}
      {auth.error && (
        <span role="alert" className="max-w-56 text-sm">
          {auth.error}
        </span>
      )}
    </div>
  );
}
