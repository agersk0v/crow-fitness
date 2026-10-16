"use client";
import type { ReactNode } from "react";
import { useReducedMotion } from "motion/react";
import { useSortable } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";

export function SortableExercise({
  id,
  label,
  disabled,
  children,
  kind = "exercise",
}: {
  id: string;
  label: string;
  disabled: boolean;
  children: ReactNode;
  kind?: "exercise" | "set";
}) {
  const reducedMotion = useReducedMotion();
  const {
    attributes,
    listeners,
    setNodeRef,
    setActivatorNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({
    id,
    disabled,
    transition: { duration: 180, easing: "ease" },
  });
  return (
    <div
      ref={(node) => {
        setNodeRef(node);
        setActivatorNodeRef(node);
      }}
      {...listeners}
      role="group"
      tabIndex={disabled ? -1 : 0}
      aria-disabled={disabled}
      aria-describedby={attributes["aria-describedby"]}
      aria-roledescription={`sortable ${kind}`}
      aria-label={`Reorder ${label}`}
      data-dragging={isDragging || undefined}
      style={{
        transform: CSS.Transform.toString(transform),
        transition: reducedMotion ? undefined : transition,
      }}
      className={`relative min-w-0 rounded-xl focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring [&_input]:cursor-text [&_select]:cursor-default [&_button]:cursor-pointer [&_label]:cursor-default ${disabled ? "" : "cursor-grab active:cursor-grabbing"} ${isDragging ? "z-10 rounded-xl bg-card shadow-lg ring-2 ring-primary" : ""}`}
    >
      {children}
    </div>
  );
}
