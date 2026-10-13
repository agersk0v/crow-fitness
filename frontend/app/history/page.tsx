import Link from "next/link";
import { ThemeToggle } from "@/components/theme-toggle";
import { HistoryTabs } from "@/components/history-tabs";
export default async function HistoryPage({
  searchParams,
}: {
  searchParams: Promise<{ tab?: string | string[] }>;
}) {
  const query = await searchParams;
  const selected = query.tab === "sets" ? "sets" : "sessions";
  return (
    <main className="mx-auto min-h-screen max-w-5xl px-6 py-8 sm:px-12">
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-border pb-6">
        <nav
          aria-label="Main navigation"
          className="flex flex-wrap gap-4 text-sm text-highlight"
        >
          <Link href="/">Home</Link>
          <Link href="/templates">Templates</Link>
          <Link href="/exercises">Exercises</Link>
        </nav>
        <ThemeToggle />
      </header>
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
