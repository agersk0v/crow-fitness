import Link from "next/link";
import { CompletedSessionDetail } from "@/components/completed-session-detail";
export default async function CompletedSessionPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return (
    <main className="mx-auto min-h-screen max-w-3xl px-6 py-8 sm:px-12">
      <Link
        href="/history?tab=sessions"
        className="text-sm text-highlight underline focus-visible:outline-2 focus-visible:outline-ring"
      >
        ← Workout history
      </Link>
      <div className="py-10">
        <CompletedSessionDetail key={id} id={id} />
      </div>
    </main>
  );
}
