// The items of a text box that holds one item per line: each line trimmed, the empty ones dropped.
export function toLines(text: string): string[] {
  return text
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line !== "");
}

// The text of a text box that holds the items, one per line.
export function fromLines(items: readonly string[]): string {
  return items.join("\n");
}
