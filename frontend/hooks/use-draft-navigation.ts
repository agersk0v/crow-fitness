"use client";
import { useEffect, useRef } from "react";

// Guard ordinary in-app links and browser navigation without changing Next's
// routing. Modified/new-tab links leave the current draft mounted.
export function useDraftNavigation(
  dirty: boolean,
  saving: boolean,
  enabled = true,
) {
  const state = useRef({ dirty, saving });
  state.current = { dirty, saving };
  function canLeave() {
    if (state.current.saving) return false;
    if (!state.current.dirty) return true;
    const confirmed = window.confirm("Discard unsaved template changes?");
    if (confirmed) state.current.dirty = false;
    return confirmed;
  }
  useEffect(() => {
    if (!enabled) return;
    function click(event: MouseEvent) {
      if (
        event.button !== 0 ||
        event.ctrlKey ||
        event.metaKey ||
        event.shiftKey ||
        event.altKey
      )
        return;
      const link =
        event.target instanceof Element
          ? event.target.closest("a[href]")
          : null;
      if (
        !(link instanceof HTMLAnchorElement) ||
        (link.target && link.target !== "_self") ||
        link.hasAttribute("download") ||
        link.origin !== location.origin ||
        link.href === location.href ||
        link.getAttribute("href")?.startsWith("#")
      )
        return;
      if (!canLeave()) {
        event.preventDefault();
        event.stopImmediatePropagation();
      }
    }
    function unload(event: BeforeUnloadEvent) {
      if (state.current.dirty || state.current.saving) {
        event.preventDefault();
        event.returnValue = "";
      }
    }
    document.addEventListener("click", click, true);
    window.addEventListener("beforeunload", unload);
    return () => {
      document.removeEventListener("click", click, true);
      window.removeEventListener("beforeunload", unload);
    };
  }, [enabled]);
  useEffect(() => {
    if (!enabled || (!dirty && !saving)) return;
    const marker = "template-draft";
    history.pushState(
      { ...history.state, draftGuard: marker },
      "",
      location.href,
    );
    let leaving = false;
    function back(event: PopStateEvent) {
      if (leaving || event.state?.draftGuard === marker) return;
      if (canLeave()) {
        leaving = true;
        history.back();
      } else history.forward();
    }
    window.addEventListener("popstate", back);
    return () => {
      window.removeEventListener("popstate", back);
      // Remove the extra same-page entry after a successful save/revert.
      if (
        !state.current.dirty &&
        !state.current.saving &&
        history.state?.draftGuard === marker
      )
        history.back();
    };
  }, [enabled, dirty || saving]);
  return canLeave;
}
