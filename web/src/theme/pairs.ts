export interface Pair {
  fg: string;
  bg: string;
  min: 3 | 4.5;
}

function text(fg: string, bg: string): Pair {
  return { fg, bg, min: 4.5 };
}

export const pairs: readonly Pair[] = [
  text("fg", "bg"),
  text("surface-fg", "surface"),
  text("fg", "surface"),
  text("sidebar-fg", "sidebar"),
  text("sidebar-fg", "bg"),
  text("sidebar-active-fg", "sidebar-active"),
  text("muted-fg", "surface"),
  text("muted-fg", "bg"),
  text("accent", "surface"),
  text("accent", "bg"),
  text("accent-fg", "accent"),
  text("healthy", "surface"),
  text("healthy", "bg"),
  text("attention", "surface"),
  text("attention", "bg"),
  text("critical", "surface"),
  text("critical", "bg"),
  text("neutral", "surface"),
  text("neutral", "bg"),
  text("critical-surface-fg", "critical-surface"),
  text("info-surface-fg", "info-surface"),
  { fg: "input-border", bg: "surface", min: 3 },
  { fg: "accent", bg: "surface", min: 3 },
  { fg: "accent", bg: "bg", min: 3 },
];
