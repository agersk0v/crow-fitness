export type Exercise = {
  id: string;
  name: string;
  description: string;
  muscle_groups: string[];
};
export type TemplateReference = { id: string; name: string; archived: boolean };
export function muscleGroupLabel(group: string) {
  return group.charAt(0).toUpperCase() + group.slice(1);
}

export type ExercisePage = {
  items: Exercise[];
  muscle_groups: string[];
  page: number;
  page_size: number;
  total_items: number;
  total_pages: number;
};
