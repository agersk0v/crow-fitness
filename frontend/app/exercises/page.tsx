import Link from "next/link";
import { PageEntrance } from "@/components/page-entrance";
import { ThemeToggle } from "@/components/theme-toggle";
import { ExerciseLibrary } from "@/components/exercise-library";

export default function ExercisesPage() {
  return (
    <main className="mx-auto min-h-screen max-w-5xl px-6 py-8 sm:px-12">
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-border pb-6">
        <Link
          href="/"
          className="text-sm font-medium text-highlight hover:text-highlight/80"
        >
          ← Back to home
        </Link>
        <Link
          href="/templates"
          className="text-sm font-medium text-highlight hover:text-highlight/80"
        >
          Templates
        </Link>
        <ThemeToggle />
      </header>
      <PageEntrance className="py-10 sm:py-16">
        <h1 className="text-4xl font-semibold tracking-tight sm:text-5xl">
          Exercises
        </h1>
        <p className="mt-4 max-w-xl text-muted-foreground">
          Browse your exercises and add the movements you train.
        </p>
        <div className="mt-8">
          <ExerciseLibrary />
        </div>
      </PageEntrance>
    </main>
  );
}
