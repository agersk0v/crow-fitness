import Link from "next/link";
import { ExerciseLibrary } from "@/components/exercise-library";
import { staticExercises } from "@/lib/static-exercises";

export default function ExercisesPage() {
  return (
    <main className="mx-auto min-h-screen max-w-5xl px-6 py-8 sm:px-12">
      <header className="border-b border-white/10 pb-6">
        <Link
          href="/"
          className="text-sm font-medium text-lime-300 hover:text-lime-200"
        >
          ← Back to home
        </Link>
      </header>
      <section className="py-10 sm:py-16">
        <h1 className="text-4xl font-semibold tracking-tight sm:text-5xl">
          Exercises
        </h1>
        <p className="mt-4 max-w-xl text-slate-400">
          Browse your exercises and add the movements you train.
        </p>
        <p className="mb-8 mt-4 rounded-lg border border-lime-300/20 bg-lime-300/5 px-4 py-3 text-sm text-slate-300">
          Preview with sample exercises. Additions last until you refresh this
          page.
        </p>
        <ExerciseLibrary initialExercises={staticExercises} />
      </section>
    </main>
  );
}
