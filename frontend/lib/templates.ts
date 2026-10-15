export type TemplateSummary = {
  id: string;
  name: string;
  description: string;
  archived: boolean;
};
export type TemplateExercise = {
  exercise_id: string;
  name: string;
  position: number;
  set_count: number;
  rep_min: number | null;
  rep_max: number | null;
  target_rir: number | null;
};
export type Template = TemplateSummary & { exercises: TemplateExercise[] };
