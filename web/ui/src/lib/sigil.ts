export type Sigil = "Aa" | "#" | "01" | "◷" | "()" | "[]" | "?";

const norm = (t: string) => t.trim().toLowerCase();

export function sigilFor(rawType: string): Sigil {
  const t = norm(rawType);
  if (t === "") return "?";

  // arrays first: repeated X, vector<X>, list, [], Mapped[list]
  if (/^repeated\b/.test(t) || /vector<|\blist\b|\[\]$|array/.test(t) || t.includes("mapped[list]")) {
    return "[]";
  }
  // booleans
  if (/\bbool(ean)?\b/.test(t)) return "01";
  // timestamps
  if (/\b(timestamp|datetime|date)\b/.test(t)) return "◷";
  // numbers
  if (/\b(int\d*|uint\d*|float|double|number|decimal|numeric|real|bigint|smallint)\b/.test(t) ||
      /mapped\[(int|float|decimal)/.test(t)) {
    return "#";
  }
  // strings
  if (/\b(string|str|varchar|char|text)\b/.test(t) || /mapped\[str/.test(t)) {
    return "Aa";
  }
  // objects/messages/opaque structured (Geometry, Command, Mapped[dict], Mapped[uuid...])
  if (/^[a-z_][a-z0-9_.:]*$/.test(t) || /mapped\[(dict|uuid|json)/.test(t) || /object|message|struct/.test(t)) {
    return "()";
  }
  return "?";
}
