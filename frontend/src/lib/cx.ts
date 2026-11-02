/** Joins class names, skipping falsy ones: cx(s.card, active && s.active). */
export function cx(...names: (string | false | null | undefined)[]) {
  return names.filter(Boolean).join(" ");
}
