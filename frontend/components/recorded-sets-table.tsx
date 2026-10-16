import { techniqueLabel, type SavedSet } from "@/lib/sessions";

export function RecordedSetsTable({
  sets,
  caption,
}: {
  sets: SavedSet[];
  caption: string;
}) {
  return (
    <table className="w-full table-fixed break-words text-left text-xs tabular-nums sm:text-sm">
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
          <th scope="col" className="py-3 font-medium">
            Technique
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
              {set.rir ?? <span aria-label="Not recorded">—</span>}
            </td>
            <td className="py-3">
              <span
                className={
                  set.change?.technique === "improved"
                    ? "text-green-700 dark:text-green-400"
                    : set.change?.technique === "regressed"
                      ? "text-red-700 dark:text-red-400"
                      : undefined
                }
                aria-label={
                  set.change?.technique && set.change.technique !== "unchanged"
                    ? `Technique ${set.change.technique} from ${techniqueLabel(set.change.previous_technique)} to ${techniqueLabel(set.technique)}`
                    : undefined
                }
              >
                {techniqueLabel(set.technique)}
                {set.change?.technique === "improved"
                  ? " ↑"
                  : set.change?.technique === "regressed"
                    ? " ↓"
                    : ""}
              </span>
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
