"use client";

import { useEntranceAnimation } from "@/hooks/use-entrance-animation";
import type { Exercise } from "@/lib/static-exercises";

export function ExerciseRow({
  exercise,
  animateEntrance,
}: {
  exercise: Exercise;
  animateEntrance: boolean;
}) {
  const scope = useEntranceAnimation<HTMLLIElement>(animateEntrance);
  return (
    <li ref={scope} className="break-words px-5 py-4 font-medium">
      {exercise.name}
    </li>
  );
}
