export interface Pair {
  fg: string;
  bg: string;
  min: 3 | 4.5;
}

function text(fg: string, bg: string): Pair {
  return { fg, bg, min: 4.5 };
}

function line(fg: string, bg: string): Pair {
  return { fg, bg, min: 3 };
}

// The texts a peak cell carries, its value and its share with the shape of their status, on Depth 1 and on the
// stripe of the hazard hatch over it. Depth 2 and Depth 3 lie only under the gauge, which holds no text.
const onDepth = ["depth-1", "hazard-stripe"].flatMap((depth) =>
  ["fg", "muted-fg", "healthy", "attention", "critical", "neutral"].map((fg) => text(fg, depth)),
);

export const pairs: readonly Pair[] = [
  text("fg", "bg"),
  text("bg", "fg"),
  text("surface-fg", "surface"),
  text("fg", "surface"),
  text("sidebar-fg", "sidebar"),
  text("sidebar-fg", "bg"),
  text("sidebar-active-fg", "sidebar-active"),
  text("fg", "info-surface"),
  text("muted-fg", "info-surface"),
  text("healthy", "info-surface"),
  text("attention", "info-surface"),
  text("critical", "info-surface"),
  text("neutral", "info-surface"),
  text("muted-fg", "surface"),
  text("muted-fg", "bg"),
  text("accent", "surface"),
  text("accent", "bg"),
  text("action-fg", "action"),
  text("healthy", "surface"),
  text("healthy", "bg"),
  text("attention", "surface"),
  text("attention", "bg"),
  text("critical", "surface"),
  text("critical", "bg"),
  text("neutral", "surface"),
  text("neutral", "bg"),
  text("surface", "critical"),
  text("critical-surface-fg", "critical-surface"),
  text("info-surface-fg", "info-surface"),
  ...onDepth,
  line("input-border", "surface"),
  line("input-border", "bg"),
  line("accent", "surface"),
  line("accent", "bg"),
  line("accent", "info-surface"),
  line("accent", "sidebar"),
  line("accent", "sidebar-active"),
  line("neatline", "bg"),
  line("neatline", "surface"),
  line("neatline", "sidebar"),
  line("neatline", "sidebar-active"),
  line("action", "surface"),
  line("action", "bg"),
  line("critical", "bg"),
  line("critical", "depth-3"),
  line("attention", "depth-3"),
];
