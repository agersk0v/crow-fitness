import { HistoryTabs } from "@/components/history-tabs";
export default async function HistoryPage({
  searchParams,
}: {
  searchParams: Promise<{ tab?: string | string[] }>;
}) {
  const query = await searchParams;
  const selected = query.tab === "sessions" ? "sessions" : "sets";
  return (
    <main className="mx-auto min-h-screen max-w-5xl px-6 py-8 sm:px-12">
      <div className="space-y-6 py-10">
        <h1 className="text-4xl font-semibold">Workout history</h1>
        <p className="text-muted-foreground">
          Completed workouts and their saved sets, newest workouts first.
        </p>
        <HistoryTabs selected={selected} />
      </div>
    </main>
  );
}
