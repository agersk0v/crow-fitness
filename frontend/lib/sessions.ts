import type { TemplateExercise } from "@/lib/templates";

export type Session = {
  id: string;
  template_id: string;
  name: string;
  started_at: string;
  exercises: TemplateExercise[];
  sets: SavedSet[];
  completed_at: string | null;
};

export type Technique = "good" | "decent" | "bad";

export function techniqueLabel(value: Technique | null | undefined) {
  return value
    ? { good: "Good", decent: "Decent", bad: "Bad" }[value]
    : "Not recorded";
}

export type SavedSet = {
  technique: Technique;
  rir?: string | null;
  change?: {
    weight: string;
    reps: string;
    technique?: "improved" | "regressed" | "unchanged";
    previous_technique?: Technique;
  };
  id: string;
  exercise_position: number;
  weight: string;
  reps: number;
};
export type CompletedSet = SavedSet & {
  session_id: string;
  exercise_name: string;
  exercise_id: string;
  session_name: string;
  completed_at: string;
};

export type CompletedSession = {
  id: string;
  name: string;
  completed_at: string;
};
