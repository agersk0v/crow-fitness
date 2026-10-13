import { PageEntrance } from "@/components/page-entrance";
import { HomeWorkout } from "@/components/home-workout";
import { ConnectionStatus } from "@/components/connection-status";

// Read the temporary testing flag at runtime, including production Docker runs.
export const dynamic = "force-dynamic";

export default function Home() {
  return (
    <main className="mx-auto flex min-h-screen max-w-5xl flex-col px-6 py-8 sm:px-12">
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
            Create exercises, reusable workout templates, and start a workout.
            Save your sets and complete your workout to keep the results.
          </p>
        </div>
        <HomeWorkout resetEnabled={process.env.ENABLE_DEV_RESET === "true"} />
        <ConnectionStatus />
      </PageEntrance>

      <footer className="border-t border-border pt-6 text-sm text-muted-foreground">
        One person. No accounts. Built for keeping things simple.
      </footer>
    </main>
  );
}
