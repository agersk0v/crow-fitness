import Link from "next/link";
import { TemplateDetail } from "@/components/template-detail";

export default async function TemplatePage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return (
    <main className="[&_button:enabled]:cursor-pointer [&_a[href]]:cursor-pointer mx-auto min-h-screen max-w-3xl px-6 py-8 sm:px-12">
      <Link
        href="/templates"
        className="text-sm text-highlight underline focus-visible:outline-2 focus-visible:outline-ring"
      >
        ← Templates
      </Link>
      <div className="py-10">
        <TemplateDetail key={id} id={id} />
      </div>
    </main>
  );
}
