import Link from "next/link";
import { PageEntrance } from "@/components/page-entrance";
import { ThemeToggle } from "@/components/theme-toggle";
import { TemplateLibrary } from "@/components/template-library";

export default function TemplatesPage() {
  return (
    <main className="mx-auto min-h-screen max-w-5xl px-6 py-8 sm:px-12">
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-border pb-6">
        <nav
          aria-label="Main navigation"
          className="flex gap-4 text-sm text-highlight"
        >
          <Link href="/">Home</Link>
          <Link href="/exercises">Exercises</Link>
        </nav>
        <ThemeToggle />
      </header>
      <PageEntrance className="py-10 sm:py-16">
        <h1 className="text-4xl font-semibold tracking-tight sm:text-5xl">
          Workout templates
        </h1>
        <p className="mb-8 mt-4 text-muted-foreground">
          Plan your exercises and working sets, then save a routine to use
          again.
        </p>
        <TemplateLibrary />
      </PageEntrance>
    </main>
  );
}
