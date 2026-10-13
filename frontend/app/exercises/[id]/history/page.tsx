import Link from "next/link";
import { ThemeToggle } from "@/components/theme-toggle";
import { ExerciseHistory } from "@/components/exercise-history";
export default async function ExerciseHistoryPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return (
    <main className="mx-auto min-h-screen max-w-3xl px-6 py-8 sm:px-12">
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-border pb-6">
        <nav
          aria-label="Main navigation"
          className="flex flex-wrap gap-4 text-sm text-highlight"
        >
          <Link href="/exercises">← Exercises</Link>
          <Link href="/history">Workout history</Link>
          <Link href="/templates">Templates</Link>
          <Link href="/">Home</Link>
        </nav>
        <ThemeToggle />
      </header>
      <div className="py-10">
        <ExerciseHistory key={id} id={id} />
      </div>
    </main>
  );
}
