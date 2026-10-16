export const historyKeys = [
  "q",
  "exercise",
  "template",
  "session",
  "from",
  "to",
] as const;
export function historyCriteria(params: URLSearchParams) {
  return Object.fromEntries(
    historyKeys.map((key) => [key, params.get(key) ?? ""]),
  ) as Record<(typeof historyKeys)[number], string>;
}
export function historyFilterError(params: URLSearchParams): string {
  for (const [key] of params) {
    if (!["tab", ...historyKeys].includes(key))
      return "Unknown history filter. Apply valid criteria or clear filters.";
    if (params.getAll(key).length !== 1)
      return "Use each history parameter once.";
  }
  const { exercise, template, session, from, to } = historyCriteria(params);
  if (
    exercise &&
    (!/^[1-9][0-9]*$/.test(exercise) ||
      BigInt(exercise) > BigInt("9223372036854775807"))
  )
    return "Choose a valid exercise ID.";
  if (
    template &&
    (!/^[1-9][0-9]*$/.test(template) ||
      BigInt(template) > BigInt("9223372036854775807"))
  )
    return "Choose a valid template ID.";
  if (
    session &&
    (!/^[1-9][0-9]*$/.test(session) ||
      BigInt(session) > BigInt("9223372036854775807"))
  )
    return "Choose a valid session ID.";
  for (const date of [from, to]) {
    if (
      date &&
      (!/^\d{4}-\d{2}-\d{2}$/.test(date) ||
        date.startsWith("0000") ||
        !Number.isFinite(Date.parse(date)) ||
        new Date(date).toISOString().slice(0, 10) !== date)
    )
      return "Use valid dates in YYYY-MM-DD format.";
  }
  if (from && to && from > to) return "From date must be on or before To date.";
  return "";
}
export function historyQuery(criteria: Record<string, string>, tab?: string) {
  const params = new URLSearchParams();
  if (tab === "sessions") params.set("tab", "sessions");
  for (const key of historyKeys) {
    const value = key === "q" ? criteria[key]?.trim() : criteria[key];
    if (value) params.set(key, value);
  }
  return params;
}
