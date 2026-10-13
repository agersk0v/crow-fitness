import Link from "next/link";
import { ExerciseHistory } from "@/components/exercise-history";
export default async function ExerciseHistoryPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return (
    <main className="mx-auto min-h-screen max-w-3xl px-6 py-8 sm:px-12">
      <Link
        href="/exercises"
        className="text-sm text-highlight underline focus-visible:outline-2 focus-visible:outline-ring"
      >
        ← Exercises
      </Link>
      <div className="py-10">
        <ExerciseHistory key={id} id={id} />
      </div>
    </main>
  );
}
