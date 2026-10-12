import Link from "next/link";
import { PageEntrance } from "@/components/page-entrance";
import { ThemeToggle } from "@/components/theme-toggle";
import { ConnectionStatus } from "@/components/connection-status";

export default function Home() {
  return (
    <main className="mx-auto flex min-h-screen max-w-5xl flex-col px-6 py-8 sm:px-12">
      <header className="flex flex-wrap items-center gap-3 border-b border-border pb-6">
        <span className="flex size-10 items-center justify-center rounded-xl bg-primary text-lg font-bold text-primary-foreground">
          C
        </span>
        <span className="text-lg font-semibold tracking-tight">
          Crow Fitness
        </span>
        <span className="ml-auto rounded-full border border-border px-3 py-1 text-xs text-muted-foreground">
          Local preview
        </span>
        <ThemeToggle />
      </header>

      <PageEntrance className="flex flex-1 flex-col justify-center gap-8 py-16">
        <div className="max-w-2xl">
          <p className="mb-4 text-sm font-medium uppercase tracking-widest text-highlight">
            Your training. Your progress.
          </p>
          <h1 className="text-5xl font-semibold leading-tight tracking-tight sm:text-7xl">
            A simple place
            <br />
            to track your workouts.
          </h1>
          <p className="mt-6 max-w-xl text-lg leading-relaxed text-muted-foreground">
            Workout templates, set logging, and exercise history are coming
            next. This first version checks that your local setup is connected.
          </p>
        </div>
        <Link
          href="/exercises"
          className="self-start rounded-lg bg-primary px-5 py-3 font-semibold text-primary-foreground hover:bg-primary/90"
        >
          Browse exercises
        </Link>
        <ConnectionStatus />
      </PageEntrance>

      <footer className="border-t border-border pt-6 text-sm text-muted-foreground">
        One person. No accounts. Built for keeping things simple.
      </footer>
    </main>
  );
}
