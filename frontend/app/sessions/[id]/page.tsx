import { SessionDetail } from "@/components/session-detail";

export default async function SessionPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  return (
    <main className="mx-auto min-h-screen max-w-3xl px-6 py-8 sm:px-12 [&_button:enabled]:cursor-pointer [&_a[href]]:cursor-pointer">
      <div className="py-10">
        <SessionDetail key={id} id={id} />
      </div>
    </main>
  );
}
