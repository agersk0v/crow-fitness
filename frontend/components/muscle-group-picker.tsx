"use client";
import { useId } from "react";
import { muscleGroupLabel } from "@/lib/exercises";

export function MuscleGroupPicker({
  groups,
  selected,
  onChange,
  error,
}: {
  groups: string[];
  selected: string[];
  onChange: (groups: string[]) => void;
  error?: string;
}) {
  const id = useId();
  return (
    <fieldset
      aria-describedby={error ? `${id}-error` : undefined}
      className="space-y-3"
    >
      <legend className="text-sm font-medium">Muscle groups (optional)</legend>
      <div className="flex flex-wrap gap-2">
        {groups.map((group) => (
          <label
            key={group}
            className="flex min-h-10 cursor-pointer items-center gap-2 rounded-lg border border-border px-3 py-2 text-sm"
          >
            <input
              type="checkbox"
              checked={selected.includes(group)}
              onChange={(event) =>
                onChange(
                  event.target.checked
                    ? [...selected, group]
                    : selected.filter((item) => item !== group),
                )
              }
              className="size-4 accent-primary"
            />
            {muscleGroupLabel(group)}
          </label>
        ))}
      </div>
      {error && (
        <p id={`${id}-error`} className="text-sm text-destructive">
          {error}
        </p>
      )}
    </fieldset>
  );
}
