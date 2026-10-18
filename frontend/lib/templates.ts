export type TemplateSummary = {
  id: string;
  name: string;
  description: string;
  archived: boolean;
};
export type ExerciseChoice = {
  exercise_id: string;
  name: string;
  set_count: number;
  rep_min: number | null;
  rep_max: number | null;
  target_rir: number | null;
};
export type TemplateExercise = {
  alternatives?: ExerciseChoice[];
  choices?: ExerciseChoice[];
  selection_revision?: number;
  exercise_id: string;
  name: string;
  position: number;
  set_count: number;
  rep_min: number | null;
  rep_max: number | null;
  target_rir: number | null;
};
export type Template = TemplateSummary & { exercises: TemplateExercise[] };
