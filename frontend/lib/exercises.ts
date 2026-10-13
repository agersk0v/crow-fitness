export type Exercise = {
  id: string;
  name: string;
  muscle_groups: string[];
};
export type TemplateReference = { id: string; name: string };
export function muscleGroupLabel(group: string) {
  return group.charAt(0).toUpperCase() + group.slice(1);
}
