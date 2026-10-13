import type { TemplateExercise } from "@/lib/templates";

export type Session = {
  id: string;
  template_id: string;
  name: string;
  started_at: string;
  exercises: TemplateExercise[];
};
