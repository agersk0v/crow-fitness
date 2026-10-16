import type { SavedSet } from "@/lib/sessions";

export function RecordedSetsTable({
  sets,
  caption,
}: {
  sets: SavedSet[];
  caption: string;
}) {
  return (
    <table className="w-full table-fixed text-left text-sm tabular-nums">
      <caption className="sr-only">{caption}</caption>
      <thead>
        <tr className="border-b border-border text-muted-foreground">
          <th scope="col" className="py-3 pr-3 font-medium">
            Weight (kg)
          </th>
          <th scope="col" className="py-3 pr-3 font-medium">
            Reps
          </th>
          <th scope="col" className="py-3 font-medium">
            RIR
          </th>
        </tr>
      </thead>
      <tbody className="divide-y divide-border">
        {sets.map((set) => (
          <tr key={set.id}>
            <td className="py-3 pr-3">{set.weight}</td>
            <td className="py-3 pr-3">{set.reps}</td>
            <td className="py-3">
              <span aria-label="Not recorded">—</span>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
