import { Suspense } from "react";
import { PageEntrance } from "@/components/page-entrance";
import { ExerciseLibrary } from "@/components/exercise-library";

export default function ExercisesPage() {
  return (
    <main className="mx-auto min-h-screen max-w-5xl px-6 py-8 sm:px-12">
      <PageEntrance className="py-10 sm:py-16">
        <h1 className="text-4xl font-semibold tracking-tight sm:text-5xl">
          Exercises
        </h1>
        <p className="mt-4 max-w-xl text-muted-foreground">
          Browse your exercises and add the movements you train.
        </p>
        <div className="mt-8">
          <Suspense fallback={<p role="status">Loading exercises…</p>}>
            <ExerciseLibrary />
          </Suspense>
        </div>
      </PageEntrance>
    </main>
  );
}
