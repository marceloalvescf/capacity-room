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

// The texts a peak cell carries, its value and its share with the shape of their status, on the tint of an
// attention or a critical peak.
const onTint = ["attention-surface", "critical-surface"].flatMap((tint) =>
  ["fg", "muted-fg", "healthy", "attention", "critical", "neutral"].map((fg) => text(fg, tint)),
);

// The fill of a proof bar, in the colour of its status or in ink, and its reference tick, against the track.
const onTrack = ["critical", "attention", "healthy", "neatline"].map((fg) => line(fg, "track"));

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
  ...onTint,
  ...onTrack,
  // A control under the pointer: a secondary button, a released segment or severity toggle with its status mark,
  // a period choice, its stroke, and the focus ring around it.
  text("surface-fg", "hover"),
  text("fg", "hover"),
  text("muted-fg", "hover"),
  text("healthy", "hover"),
  text("attention", "hover"),
  text("critical", "hover"),
  text("neutral", "hover"),
  line("input-border", "hover"),
  line("neatline", "hover"),
  line("accent", "hover"),
  text("action-fg", "action-hover"),
  line("action-hover", "bg"),
  line("action-hover", "surface"),
  text("surface", "critical-hover"),
  line("critical-hover", "bg"),
  line("critical-hover", "surface"),
  text("sidebar-active-fg", "sidebar-hover"),
  line("accent", "sidebar-hover"),
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
];
