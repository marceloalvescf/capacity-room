---
name: CapacityRoom
description: Kubernetes capacity reports read as a hydrographic chart reads water; room left is depth under the keel.
colors:
  chart-ground: "#f5f9fb"
  chart-ink: "#0e1a22"
  sheet: "#ffffff"
  sheet-ink: "#0e1a22"
  shallows: "#deedf4"
  shallows-ink: "#26404f"
  shallows-active: "#bcdbe9"
  shallows-active-ink: "#0b1f2b"
  muted-ink: "#4a5c68"
  hairline: "#cfdbe2"
  neatline: "#22323d"
  input-border: "#6c808d"
  sounding-blue: "#14375c"
  action-ink: "#22323d"
  action-ink-fg: "#ffffff"
  sea-green: "#17694f"
  ochre: "#7f4d00"
  chart-magenta: "#a3135c"
  slate-neutral: "#485a66"
  hazard-surface: "#fbe8f1"
  hazard-border: "#e9b1cd"
  hazard-surface-ink: "#76104a"
  info-surface: "#e3f0f7"
  info-surface-ink: "#1b4870"
  depth-1: "#d3e8f2"
  depth-2: "#bcdbea"
  depth-3: "#9ccae0"
typography:
  display:
    fontFamily: "JetBrains Mono Variable, ui-monospace, monospace"
    fontSize: "3.5rem"
    fontWeight: 600
    lineHeight: 1
    letterSpacing: "-0.025em"
  headline:
    fontFamily: "Public Sans Variable, system-ui, sans-serif"
    fontSize: "1.25rem"
    fontWeight: 600
    lineHeight: 1.4
  title:
    fontFamily: "Public Sans Variable, system-ui, sans-serif"
    fontSize: "1.125rem"
    fontWeight: 600
    lineHeight: 1.55
  body:
    fontFamily: "Public Sans Variable, system-ui, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 400
    lineHeight: 1.43
  data:
    fontFamily: "JetBrains Mono Variable, ui-monospace, monospace"
    fontSize: "0.875rem"
    fontWeight: 400
    lineHeight: 1.43
  label:
    fontFamily: "Public Sans Variable, system-ui, sans-serif"
    fontSize: "0.6875rem"
    fontWeight: 600
    lineHeight: 1.45
    letterSpacing: "0.08em"
rounded:
  hairline: "2px"
spacing:
  "0.5": "2px"
  "1": "4px"
  "1.5": "6px"
  "2": "8px"
  "3": "12px"
  "4": "16px"
  "6": "24px"
  "10": "40px"
components:
  button-primary:
    backgroundColor: "{colors.action-ink}"
    textColor: "{colors.action-ink-fg}"
    typography: "{typography.body}"
    rounded: "{rounded.hairline}"
    padding: "0 12px"
    height: "36px"
  button-secondary:
    backgroundColor: "{colors.sheet}"
    textColor: "{colors.sheet-ink}"
    typography: "{typography.body}"
    rounded: "{rounded.hairline}"
    padding: "0 12px"
    height: "36px"
  button-danger:
    backgroundColor: "{colors.chart-magenta}"
    textColor: "{colors.sheet}"
    typography: "{typography.body}"
    rounded: "{rounded.hairline}"
    padding: "0 12px"
    height: "36px"
  icon-button:
    backgroundColor: "{colors.sheet}"
    textColor: "{colors.sheet-ink}"
    rounded: "{rounded.hairline}"
    size: "36px"
  input:
    backgroundColor: "{colors.sheet}"
    textColor: "{colors.sheet-ink}"
    typography: "{typography.body}"
    rounded: "{rounded.hairline}"
    padding: "0 12px"
    height: "36px"
  segmented-checked:
    backgroundColor: "{colors.action-ink}"
    textColor: "{colors.action-ink-fg}"
    rounded: "{rounded.hairline}"
    padding: "0 16px"
    height: "36px"
  nav-item-active:
    backgroundColor: "{colors.shallows-active}"
    textColor: "{colors.shallows-active-ink}"
    rounded: "{rounded.hairline}"
    padding: "8px 12px"
  tab:
    textColor: "{colors.muted-ink}"
    typography: "{typography.body}"
    padding: "8px 10px"
  tab-active:
    textColor: "{colors.chart-ink}"
    padding: "8px 10px"
  table-container:
    backgroundColor: "{colors.sheet}"
    textColor: "{colors.sheet-ink}"
    rounded: "{rounded.hairline}"
  table-row-open:
    backgroundColor: "{colors.info-surface}"
  peak-cell-attention:
    backgroundColor: "{colors.depth-1}"
  alert-error:
    backgroundColor: "{colors.hazard-surface}"
    textColor: "{colors.hazard-surface-ink}"
    rounded: "{rounded.hairline}"
    padding: "8px 12px"
  alert-notice:
    backgroundColor: "{colors.info-surface}"
    textColor: "{colors.info-surface-ink}"
    rounded: "{rounded.hairline}"
    padding: "8px 12px"
  tooltip:
    backgroundColor: "{colors.chart-ink}"
    textColor: "{colors.chart-ground}"
    rounded: "{rounded.hairline}"
    padding: "4px 8px"
  dialog:
    backgroundColor: "{colors.sheet}"
    textColor: "{colors.sheet-ink}"
    rounded: "{rounded.hairline}"
    padding: "24px"
    width: "min(36rem, calc(100vw - 2rem))"
  popover:
    backgroundColor: "{colors.sheet}"
    textColor: "{colors.sheet-ink}"
    rounded: "{rounded.hairline}"
    padding: "12px"
---

# Design System: CapacityRoom

## Overview

**Creative North Star: "The Nautical Chart"**

CapacityRoom reads a cluster the way a hydrographic chart reads water. Room left is depth under the keel. The ground is white deep water with a faint blue cast. The shell is a pale shallow-water tint closed by dark neat-lines. Graded blue depth tints appear only where room runs out. Chart magenta marks a hazard (critical), ochre marks attention, and sea-green marks healthy. The dark theme is a night chart: muted and never glowing.

The system is flat, dense, and drawn with hairlines. Every radius is 2px and nothing casts a shadow. Structure comes from neat-lines, hairline rules, and tonal tints. Names and numbers are set in JetBrains Mono, and its italic is the soundings voice of secondary numbers. Interface text is set in Public Sans. The signature drawing is the depth gauge: the cluster's nodes as a sounded depth scale, with the last band hatched in magenta once no room is left.

The visual system answers to two binding project rules. Status is a shape, then a label, then a colour, so no status is ever carried by colour alone. Every foreground and background drawn together is a listed token pair that passes a contrast test in both themes.

**Key Characteristics:**
- White chart ground, pale shallows shell, dark neat-lines.
- Hairline shapes: 2px radius everywhere, no shadows anywhere.
- Three depth tints that follow the status the report gives a value, never a threshold of the web app's own.
- Magenta hatching is the one hazard device.
- Monospace for every name and number; mono italic for secondary soundings.
- One uppercase tracked label size for table headers and small section headings.
- Motion only under reduced-motion-safe conditions, and only to sound the gauge once.

## Colors

A cool chart palette: blue-cast neutrals, three blue depth tints, and three chart status hues. Each token has a light value (frontmatter) and a night-chart value (sidecar), switched by `data-theme="dark"` on the root.

### Primary
- **Sounding Blue** (accent): links, the active tab rule, the focus ring, the current-level line of the depth gauge, and the load-line stroke of the brand disc. It is the chart's own ink for navigation, not a fill.
- **Action Ink** (action / action-fg): the filled primary button, the checked segment, and the native control accent. In light it is the neat-line ink; in dark it inverts to a pale slate with dark text.

### Secondary
- **Chart Magenta** (critical): the critical status, field error text, the danger button, and the hazard hatching. Critical labels are set bold.
- **Ochre** (attention): the attention status.
- **Sea Green** (healthy): the healthy status.
- **Slate Neutral** (neutral): the undefined and unavailable statuses.

### Tertiary
- **Depth 1 / Depth 2 / Depth 3** (depth-1..3): graded water. Depth 1 tints a peak cell in attention. Depth 2 is the ground of the hazard hatch on a critical peak cell. Depth 3 lies only under the gauge and carries no text. The gauge tints its filled bands from Depth 1 to Depth 3 toward the maximum.

### Neutral
- **Chart Ground** (bg): page background.
- **Chart Ink** (fg): body text; also the tooltip ground.
- **Sheet** (surface / surface-fg): tables, inputs, secondary buttons, dialogs, popovers, menus.
- **Shallows** (sidebar / sidebar-fg): the side shell and the sign-in header.
- **Shallows Active** (sidebar-active / sidebar-active-fg): the current navigation entry and a highlighted menu item.
- **Muted Ink** (muted-fg): captions, meta lines, labels, secondary values.
- **Hairline** (border): row rules, table frames, the tab baseline.
- **Neatline** (neatline): the shell's closing lines, the frame of every floating layer, the top rule of the overview verdict, and the gauge outline and ticks.
- **Input Border** (input-border): field and secondary-button strokes, held at 3:1.
- **Scrim** (`rgb(14 26 34 / 0.45)` light, `rgb(0 0 0 / 0.65)` dark): the overlay behind a modal.
- **Hazard Surface / Border / Ink** and **Info Surface / Ink**: the error alert and the notice alert. Info Surface is also the ground of an opened table row.

### Named Rules
**The Token Pair Rule.** A colour is a token pair. Every foreground drawn on a background is listed in `web/src/theme/pairs.ts` and must reach 4.5:1 for text, or 3:1 for a large heading, an input border, a focus ring, or a line, in both themes. A new pair on screen enters the list first.

**The Status-Led Depth Rule.** A depth tint follows the status the report view gives a value. Attention takes Depth 1, critical takes the hazard hatch over Depth 2, and every other status takes none. The web app never computes a threshold of its own.

**The One Hazard Rule.** Magenta hatching (135 degrees, 1.5px lines every 6px at 12% critical over Depth 2 in a cell; 2.2px critical lines over Depth 3 in the gauge) means critical and nothing else.

## Typography

**Display Font:** JetBrains Mono Variable (with ui-monospace, monospace)
**Body Font:** Public Sans Variable (with system-ui, sans-serif)
**Label/Mono Font:** JetBrains Mono Variable for data; Public Sans for labels

**Character:** Public Sans stands upright for names of things in the interface: navigation, headings, sentences. JetBrains Mono carries every name and number of the cluster, and its italic speaks the secondary soundings: shares, counts, gauge scale numbers. Both faces are self-hosted from npm packages, because the CSP allows no outside source.

### Hierarchy
- **Display** (mono 600, 3.5rem, line-height 1, tracking -0.025em): the verdict numeral "current / maximum" of the overview. One per screen; the slash between is set in Public Sans 400 in Muted Ink.
- **Headline** (600, 1.25rem): the page title of every screen and the sign-in card.
- **Title** (600, 1.125rem): dialog titles and the brand word (700, tracking tight). Section headings inside a page step down to 1rem 600.
- **Body** (400, 0.875rem / 1.25rem): sentences, table cells, controls, captions. Small meta under a name is 0.75rem.
- **Data** (mono 400, 0.875rem): every cluster name, number, version, user name, and time zone. Count rows set it at 1.25rem 600.
- **Sounding** (mono italic): the share under a peak, counts after a status word, the minimum-node and event figures, and the gauge scale (11px).
- **Label** (600, 0.6875rem, line-height 1rem, tracking 0.08em, uppercase, Muted Ink): table headers and small section headings. A sort button inside a header takes it whole.

### Named Rules
**The Mono Names Rule.** Every name and number a cluster produced is set in JetBrains Mono. Interface words never are.

**The One Label Rule.** Uppercase tracked text has exactly one size (0.6875rem). It names a column or a section; it is never a decorative tag above a title.

## Layout

The signed-in screen is a two-column grid at `md` (768px) and above: a 16rem shallows column on the left, the working area on the right. Below `md` the shell stacks on top, closed by a neat-line underneath instead of at its side. The working area has a header row of icon buttons at the right, closed by a neat-line, and a main region with 24px padding.

Spacing follows a 4px step: 2, 4, 6, 8, 12, 16, 24, and 40px are the values in use. Controls and table cells use 12px horizontal and 8px vertical padding; stacks of sections use 16px gaps; the overview verdict uses 24px vertical padding and a 40px column gap. At `xl` (1280px) the verdict splits 4fr to 7fr: growth on the left, pressure by scope on the right.

Tables never scroll sideways. Cells wrap, names break anywhere, and the header sticks under the window top. An opened row reveals itself between the sticky header and the window bottom. Report screens have seven tabs under the title; the PDF action sits at the right of the title.

## Elevation & Depth

The system is flat. The theme resets every Tailwind shadow scale (`--shadow-*`, `--inset-shadow-*`, `--drop-shadow-*`, `--text-shadow-*`) to nothing. Depth is conveyed by three means only: neat-line frames, tonal tints (shallows, info surface, the three depth tints), and stacking order. Floating layers (menu, popover, tooltip, dialog, alert dialog) sit at z-50 above the sticky table header, framed by a Neatline border on Sheet; modals add the Scrim.

### Named Rules
**The No-Shadow Rule.** Nothing casts a shadow. A layer separates from the page by its neat-line frame alone.

## Shapes

Every radius is 2px: a near-square corner that keeps a hairline drawing from looking cut. Borders are 1px; the active tab rule is 2px; the focus ring is a 2px Sounding Blue outline with a 2px offset. The active navigation entry gets an inset 1px Neatline ring. The brand mark is a load-line (Plimsoll) disc: a 2.25px circle crossed at the centre by a Sounding Blue line. Status shapes are drawn as 12px SVGs: a filled circle (healthy), a filled triangle (attention), a filled diamond (critical), an open circle (undefined), and a dash (unavailable).

## Components

### Buttons
- **Shape:** gently squared (2px), 1px border, min height 36px, 12px horizontal padding, 0.875rem weight 500.
- **Primary:** Action Ink fill and border with Action Ink foreground.
- **Secondary (default):** Sheet fill, Input Border stroke, Sheet ink.
- **Danger:** Chart Magenta fill and border, Sheet text; for destructive confirmation.
- **Focus / Disabled:** 2px Sounding Blue outline offset 2px; disabled drops to 60% opacity with a not-allowed cursor. No hover colour shift.
- **Icon button:** a secondary button 36px square with a 20px Phosphor icon; its name is in `aria-label` and in a tooltip on hover and focus.

### Segmented control
A row of radio labels joined edge to edge (-1px overlap), 2px outer corners, 16px padding. The checked segment takes the Action Ink fill; focus outlines the segment.

### Inputs / Fields
- **Style:** Sheet fill, 1px Input Border, 2px radius, min height 36px, 12px padding. The label sits above at 0.875rem 500; a hint below in Muted Ink; an error below in Chart Magenta. A select is the browser's own `<select>`, styled the same.
- **Focus:** 2px Sounding Blue outline, 2px offset.
- **Checkbox:** native box tinted with Action Ink.

### Navigation
- **Shell:** Shallows column, brand at the top, links at 0.875rem with 8px by 12px padding. The current page takes the Shallows Active tint, weight 600, and an inset Neatline frame. The user name and version sit at the foot in mono.
- **Report tabs:** a row of 0.875rem tabs over a Hairline baseline; the active tab is Chart Ink 600 with a 2px Sounding Blue underline, the rest Muted Ink.

### Data table
The one table of the app: a Sheet container with a 1px Hairline frame and 2px corners, uppercase labels in the sticky header, Hairline rules between rows, an expander column of 48px. An opened row is tinted Info Surface and its detail row sits on Chart Ground at 24px by 16px padding. A peak cell stacks the peak in mono over its share in mono italic, and takes the depth tint its status earns.

### Status mark
Shape, then label, then colour. The 12px shape and the word share the status hue; critical words are bold. Where the label is not the status word, the status word is added for screen readers.

### Alerts
2px corners, 1px border, 8px by 12px padding, 0.875rem. Error: Hazard Surface, Hazard Border, Hazard Ink. Notice: Info Surface, Hairline border, Info Ink.

### Floating layers
Menu, popover, dialog, and alert dialog are Sheet with a 1px Neatline frame and 2px corners; a dialog pads 24px and caps at 36rem wide (28rem for a confirmation). A highlighted menu item takes Shallows Active with an inset Sounding Blue outline. The tooltip inverts: Chart Ink ground, Chart Ground text, 0.75rem.

### Depth gauge (signature)
The cluster's nodes drawn as a sounded depth scale 360px wide: one band per node up to the autoscaler maximum, inside a 1px Neatline frame. Filled bands are tinted Depth 1 to Depth 3 toward the maximum and parted by hairlines in the ground colour. A 2px Sounding Blue line marks the current level; the minimum is a dashed Neatline contour. Once no room is left, the last band is hatched magenta and a 2px Chart Magenta line closes the scale. Ticks below carry mono italic 11px soundings, at most ten. The level rises once from the left (`sounding`, 600ms, cubic-bezier(0.16, 1, 0.3, 1)), then the hazard fades in (`hazard-in`, 200ms after 600ms). Under reduced motion every part is drawn at once. The drawing is hidden from assistive technology because the numbers stand in the headline above it.

### Overview verdict
A section between a Neatline top rule and a Hairline bottom rule. Left: a label, the display numeral, the depth gauge, a caption, the worst cluster finding, mono italic secondary figures, and a link. Right: pressure by scope, one row per scope type with a Hairline top rule, the scope label as a status mark in an 8rem column, then the leading finding as an underlined link, its name and peak in mono, and its severity counts.

## Do's and Don'ts

### Do:
- **Do** take every colour from a token in `web/src/theme/theme.css` and add each new foreground and background pair to `web/src/theme/pairs.ts` before shipping it.
- **Do** draw a status as its shape first, then its word, then its colour, through the status mark.
- **Do** set every cluster name and number in JetBrains Mono, and secondary numbers in its italic.
- **Do** keep every corner at 2px and separate layers with a Neatline frame.
- **Do** tint a value only by the status the report gives it: Depth 1 for attention, the hazard hatch for critical.
- **Do** put motion only under `motion-safe:` and keep it to the single sounding of the gauge.
- **Do** style through classes and the theme's utilities; the CSP allows no inline style and no injected style element.

### Don't:
- **Don't** add a shadow, a glow, or a gradient to any surface; the theme resets every shadow scale.
- **Don't** use magenta hatching or Chart Magenta for anything but critical.
- **Don't** let a depth tint carry Depth 3 under text; Depth 3 lies only under the gauge.
- **Don't** set a label in uppercase at any size other than 0.6875rem, or use it as a decorative tag above a title.
- **Don't** compute a threshold in the web app to decide a tint or a status.
- **Don't** load a font, a stylesheet, or an icon from outside the bundle.
- **Don't** make the dark theme glow: night-chart hues stay muted.
