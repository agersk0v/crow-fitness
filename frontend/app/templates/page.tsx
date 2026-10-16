import { PageEntrance } from "@/components/page-entrance";
import { TemplateLibrary } from "@/components/template-library";

export default function TemplatesPage() {
  return (
    <main className="[&_button:enabled]:cursor-pointer [&_a[href]]:cursor-pointer mx-auto min-h-screen max-w-5xl px-6 py-8 sm:px-12">
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
