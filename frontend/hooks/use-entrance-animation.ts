"use client";

import { useEffect } from "react";
import { useAnimate } from "motion/react";

export function useEntranceAnimation<T extends HTMLElement>(enabled = true) {
  const [scope, animate] = useAnimate<T>();

  useEffect(() => {
    const element = scope.current;
    const preference = window.matchMedia("(prefers-reduced-motion: reduce)");
    if (!enabled || preference.matches || !element) return;

    // Animate only after mounting, so server-rendered content stays visible.
    const animation = animate(
      element,
      { opacity: [0.85, 1], y: [6, 0] },
      { duration: 0.18, ease: "easeOut" },
    );

    function finishAnimation() {
      animation.complete();
      element.style.removeProperty("opacity");
      element.style.removeProperty("transform");
    }
    function onPreferenceChange() {
      if (preference.matches) finishAnimation();
    }
    preference.addEventListener("change", onPreferenceChange);

    return () => {
      preference.removeEventListener("change", onPreferenceChange);
      finishAnimation();
    };
  }, [animate, enabled, scope]);

  return scope;
}
