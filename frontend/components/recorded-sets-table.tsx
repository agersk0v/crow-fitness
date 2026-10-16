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
            <td className="py-3 pr-3">
              {set.weight}{" "}
              <SetDelta value={set.change?.weight} label="Weight" unit="kg" />
            </td>
            <td className="py-3 pr-3">
              {set.reps} <SetDelta value={set.change?.reps} label="Reps" />
            </td>
            <td className="py-3">
              <span aria-label="Not recorded">—</span>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function SetDelta({
  value,
  label,
  unit = "",
}: {
  value?: string;
  label: string;
  unit?: string;
}) {
  if (!value || /^-?0(?:\.0+)?$/.test(value)) return null;
  const normalized = value.includes(".")
    ? value.replace(/0+$/, "").replace(/\.$/, "")
    : value;
  const down = normalized.startsWith("-");
  const amount = down ? normalized : `+${normalized}`;
  return (
    <span
      className={`ml-1 inline-block text-xs font-medium ${down ? "text-red-700 dark:text-red-400" : "text-green-700 dark:text-green-400"}`}
      aria-label={`${label} ${down ? "decreased" : "increased"} by ${down ? normalized.slice(1) : normalized}${unit ? ` ${unit}` : ""} from previous workout`}
    >
      {amount}
    </span>
  );
}
