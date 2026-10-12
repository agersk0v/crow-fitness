export type Exercise = {
  id: string;
  name: string;
};

export const staticExercises: Exercise[] = [
  { id: "sample-squat", name: "Squat" },
  { id: "sample-bench-press", name: "Bench press" },
  { id: "sample-deadlift", name: "Deadlift" },
];
