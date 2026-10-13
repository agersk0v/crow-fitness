import Link from "next/link";
import { ThemeToggle } from "@/components/theme-toggle";
import { CompletedSets } from "@/components/completed-sets";
export default function SetsPage() {
  return (
    <main className="mx-auto min-h-screen max-w-3xl px-6 py-8 sm:px-12">
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
        <h1 className="text-4xl font-semibold">Sets</h1>
        <p className="text-muted-foreground">
          Sets from completed workouts, newest workouts first.
        </p>
        <CompletedSets />
      </div>
    </main>
  );
}
