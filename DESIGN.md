---
name: CapacityRoom
description: Kubernetes capacity reports in which every finding is a claim set beside its proof, drawn.
colors:
  bg: "#f8f8f8"
  fg: "#0e0e0f"
  surface: "#ffffff"
  surface-fg: "#0e0e0f"
  hover: "#f0f0f1"
  sidebar: "#f8f8f8"
  sidebar-fg: "#5c5c5e"
  sidebar-hover: "#f0f0f1"
  sidebar-active: "#e7e7e8"
  sidebar-active-fg: "#0e0e0f"
  muted-fg: "#5c5c5e"
  border: "#e2e2e3"
  neatline: "#2e2e30"
  input-border: "#828284"
  track: "#eaeaeb"
  scrim: "rgb(13 14 16 / 0.4)"
  accent: "#2c55dc"
  action: "#2c55dc"
  action-hover: "#2346bd"
  action-fg: "#ffffff"
  critical: "#be1e45"
  critical-hover: "#a3183a"
  attention: "#8f5b00"
  healthy: "#24774a"
  neutral: "#5c5c5e"
  critical-surface: "#fdedf0"
  critical-border: "#f2b9c6"
  critical-surface-fg: "#8c1431"
  attention-surface: "#fdf4e3"
  info-surface: "#eef2fd"
  info-surface-fg: "#2042b0"
typography:
  display:
    fontFamily: "JetBrains Mono Variable, ui-monospace, monospace"
    fontSize: "2.25rem"
    fontWeight: 600
    lineHeight: 1
    letterSpacing: "-0.03em"
  headline:
    fontFamily: "Mona Sans Variable, system-ui, sans-serif"
    fontSize: "1.875rem"
    fontWeight: 600
    lineHeight: 1.2
    letterSpacing: "-0.025em"
  title:
    fontFamily: "Mona Sans Variable, system-ui, sans-serif"
    fontSize: "1.125rem"
    fontWeight: 600
    lineHeight: 1.556
  body:
    fontFamily: "Mona Sans Variable, system-ui, sans-serif"
    fontSize: "0.875rem"
    fontWeight: 400
    lineHeight: 1.43
  data:
    fontFamily: "JetBrains Mono Variable, ui-monospace, monospace"
    fontSize: "0.875rem"
    fontWeight: 400
    lineHeight: 1.43
  evidence-lead:
    fontFamily: "JetBrains Mono Variable, ui-monospace, monospace"
    fontSize: "1.125rem"
    fontWeight: 600
    lineHeight: 1.556
  label:
    fontFamily: "Mona Sans Variable, system-ui, sans-serif"
    fontSize: "0.6875rem"
    fontWeight: 600
    lineHeight: 1.45
    letterSpacing: "0.08em"
rounded:
  md: "6px"
  lg: "8px"
spacing:
  "0.5": "2px"
  "1": "4px"
  "1.5": "6px"
  "2": "8px"
  "3": "12px"
  "4": "16px"
  "5": "20px"
  "6": "24px"
  "7": "28px"
  "8": "32px"
components:
  button-primary:
    backgroundColor: "{colors.action}"
    textColor: "{colors.action-fg}"
    typography: "{typography.body}"
    rounded: "{rounded.md}"
    padding: "0 12px"
    height: "36px"
  button-primary-hover:
    backgroundColor: "{colors.action-hover}"
  button-secondary:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.surface-fg}"
    typography: "{typography.body}"
    rounded: "{rounded.md}"
    padding: "0 12px"
    height: "36px"
  button-secondary-hover:
    backgroundColor: "{colors.hover}"
  button-danger:
    backgroundColor: "{colors.critical}"
    textColor: "{colors.surface}"
    typography: "{typography.body}"
    rounded: "{rounded.md}"
    padding: "0 12px"
    height: "36px"
  button-danger-hover:
    backgroundColor: "{colors.critical-hover}"
  icon-button:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.surface-fg}"
    rounded: "{rounded.md}"
    size: "36px"
  input:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.surface-fg}"
    typography: "{typography.body}"
    rounded: "{rounded.md}"
    padding: "0 12px"
    height: "36px"
  segmented-track:
    backgroundColor: "{colors.hover}"
    textColor: "{colors.muted-fg}"
    rounded: "{rounded.md}"
    padding: "2px"
  segmented-checked:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.surface-fg}"
    rounded: "5px"
    padding: "0 14px"
    height: "32px"
  severity-toggle-pressed:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.surface-fg}"
    rounded: "{rounded.md}"
    padding: "0 12px"
    height: "36px"
  severity-toggle:
    textColor: "{colors.muted-fg}"
    rounded: "{rounded.md}"
    padding: "0 12px"
    height: "36px"
  nav-item:
    textColor: "{colors.sidebar-fg}"
    typography: "{typography.body}"
    rounded: "{rounded.md}"
    padding: "8px 12px"
  nav-item-hover:
    backgroundColor: "{colors.sidebar-hover}"
    textColor: "{colors.sidebar-active-fg}"
  nav-item-active:
    backgroundColor: "{colors.sidebar-active}"
    textColor: "{colors.sidebar-active-fg}"
    rounded: "{rounded.md}"
    padding: "8px 12px"
  tab:
    textColor: "{colors.muted-fg}"
    typography: "{typography.body}"
    padding: "10px 10px"
  tab-active:
    textColor: "{colors.fg}"
    padding: "10px 10px"
  claim-proof-pair:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.surface-fg}"
    rounded: "{rounded.lg}"
    padding: "16px"
  proof-track:
    backgroundColor: "{colors.track}"
    height: "8px"
  table-container:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.surface-fg}"
    rounded: "{rounded.lg}"
  table-row-open:
    backgroundColor: "{colors.info-surface}"
  peak-cell-critical:
    backgroundColor: "{colors.critical-surface}"
  peak-cell-attention:
    backgroundColor: "{colors.attention-surface}"
  finding-detail:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.surface-fg}"
    rounded: "{rounded.lg}"
    padding: "20px"
  alert-error:
    backgroundColor: "{colors.critical-surface}"
    textColor: "{colors.critical-surface-fg}"
    rounded: "{rounded.md}"
    padding: "8px 12px"
  alert-notice:
    backgroundColor: "{colors.info-surface}"
    textColor: "{colors.info-surface-fg}"
    rounded: "{rounded.md}"
    padding: "8px 12px"
  tooltip:
    backgroundColor: "{colors.fg}"
    textColor: "{colors.bg}"
    rounded: "{rounded.md}"
    padding: "4px 8px"
  dialog:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.surface-fg}"
    rounded: "{rounded.lg}"
    padding: "24px"
    width: "min(36rem, calc(100vw - 2rem))"
  popover:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.surface-fg}"
    rounded: "{rounded.lg}"
    padding: "12px"
  menu:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.surface-fg}"
    rounded: "{rounded.lg}"
    padding: "4px"
---

# Design System: CapacityRoom

## Overview

**Creative North Star: "Claim and proof"**

Every finding is a claim set beside its proof: the sentence first, then the measured value against its reference, drawn as a bar. The world is the neutral developer-tool register, won on craft rather than on ornament: one grey scale from near-white to near-black, hairline frames, a single blue for the place where the reader acts, and status colour that lives only on shapes and on the bars that prove a finding. The dark theme is the same scale inverted, not a neon observability board.

The overview opens on its answer. The severity totals stand at display size in mono beside their shapes; then the findings chart: a donut of every finding by severity over the cluster's growth card, beside a bar for each scope on one scale. The proof bar is the signature: it recurs in the overview, in the detail of a finding, and in the findings of an opened table row, and it always draws the numbers and the status the report view gives, never a threshold of the web app's own.

The visual system answers to two binding project rules. Status is a shape, then a label, then a colour, so no status is carried by colour alone. Every foreground and background drawn together is a listed token pair that passes a contrast test in both themes.

**Key Characteristics:**
- One neutral grey scale whose red, green, and blue sit within 3 units of each other, in both themes.
- One blue accent for the primary action, links, focus, and a chosen value; the current page and tab are marked in ink.
- Critical, attention, and healthy colour shapes and proof bars, never words.
- Mona Sans for words, JetBrains Mono for every name and number, no italic anywhere.
- Hairline frames, 6px controls, 8px panels; one soft shadow, for floating layers only.
- The proof bar: a value against its reference on a grey track, coloured by the finding's severity.
- Motion only under reduced-motion-safe conditions: a layer opens, a bar fills once, a button sinks.

## Colors

A neutral grey scale, one blue, three status hues, and their quiet surfaces. Each token has a light value (frontmatter) and a dark value (given here and in the sidecar), switched by `data-theme="dark"` on the root; `web/src/theme/theme.css` is the source.

### Primary
- **Signal Blue** (accent, action / action-fg): the primary button fill, a chosen period and calendar day, the native control accent, links, the focus ring, the summary of a disclosure, the pulsing ring of a list that is updating, and the line across the brand disc. Light #2c55dc with white text; dark #86a2ff with #0d0d0e text. It is used sparingly: one filled primary action per view, the rest as text links.
- **Signal Blue Hover** (action-hover): the primary fill under the pointer, deeper in light (#2346bd) and paler in dark (#a1b6ff).

### Secondary
- **Critical** (critical): the critical shape, critical proof bars, field error text, and the danger button fill (with critical-hover under the pointer). Dark #f47c95, hover #f799ac.
- **Attention** (attention): the attention shape and attention proof bars. Dark #e3a84a.
- **Healthy** (healthy): the healthy shape and healthy proof bars. Dark #5fc48f.
- **Neutral** (neutral): the shapes of the undefined and unavailable statuses. Dark #a0a0a3.

### Tertiary
- **Critical Surface / Border / Ink** (critical-surface, critical-border, critical-surface-fg): the error alert; the critical surface also tints a peak cell whose status is critical. Dark #2b1419, #6b2a38, #f6b3c1.
- **Attention Surface** (attention-surface): tints a peak cell whose status is attention. Dark #2a2112.
- **Info Surface / Ink** (info-surface, info-surface-fg): the notice alert, an opened table row, the selected entry of the findings list, and the span of a typed range in the calendar. Dark #151d33, #aebfff.

### Neutral
- **Ground** (bg): the page and the side column (sidebar shares it); also the ground of the detail row under an opened row. Dark #0d0d0e.
- **Ink** (fg, surface-fg, sidebar-active-fg): body text; the tooltip ground; the active tab underline and the outline of a pressed severity toggle. Dark #ededee.
- **Panel** (surface): tables, claim-and-proof pairs, inputs, secondary buttons, the checked segment, dialogs, popovers, menus. Dark #151516.
- **Hover** (hover, sidebar-hover): a control under the pointer, and the track of the segmented control. Dark #1e1e20.
- **Current** (sidebar-active): the current navigation entry and a highlighted menu item. Dark #27272a.
- **Muted Ink** (muted-fg, sidebar-fg): captions, meta lines, labels, navigation entries at rest, evidence names. Dark #a0a0a3.
- **Hairline** (border): row rules, panel and table frames, the frame of every floating layer, the tab baseline, the split of a pair. Dark #29292b.
- **Strong Line** (neatline): the ink of a proof bar with no status, its reference tick and its dashed floor. Dark #cdcdcf.
- **Input Border** (input-border): field and secondary-button strokes and the checked segment's edge, held at 3:1. Dark #727275.
- **Track** (track): the ground of a proof bar. Dark #262628.
- **Scrim** (scrim): behind a modal, `rgb(0 0 0 / 0.6)` in dark.

### Named Rules
**The One Scale Rule.** Every neutral is a grey whose red, green, and blue sit within 3 units of each other (2 in the light theme). No tinted neutral, no warm or cool cast, in either theme.

**The One Blue Rule.** Blue means "act here": the primary action, a link, focus, the chosen value. The current page and tab are marked in ink and the current tint, not in blue. Blue never marks a status and never decorates.

**The Colour On Shapes Rule.** A status hue colours a status shape, a proof bar, and a peak cell's tint, and nothing else of a status. The word beside a shape keeps the colour of the text around it; critical words are set semibold instead.

**The Token Pair Rule.** A colour is a token pair. Every foreground drawn on a background is listed in `web/src/theme/pairs.ts` and must reach 4.5:1 for text, or 3:1 for a line, an input border, a focus ring, or a proof bar against its track, in both themes. A new pair on screen enters the list first.

**The Status-Led Tint Rule.** A tint follows a status the view gives. A peak cell takes the worse of the status of its peak and of its share: critical takes the critical surface, attention the attention surface, every other status none. A proof bar takes the severity of its finding, or ink when it has none. The web app never computes a threshold of its own.

## Typography

**Display Font:** JetBrains Mono Variable (with ui-monospace, monospace)
**Body Font:** Mona Sans Variable (with system-ui, sans-serif)
**Label/Mono Font:** JetBrains Mono Variable for every name and number; Mona Sans for labels

**Character:** Mona Sans is a tight grotesque that carries the interface's words at 400, 500, and 600 (the brand word alone at 700); JetBrains Mono carries every name and number the cluster produced, so the answer reads as data. Both are self-hosted from `@fontsource-variable` npm packages, because the CSP allows no outside source. Mona Sans ships weights 200 to 900, JetBrains Mono 100 to 800; only the upright styles are loaded.

### Hierarchy
- **Display** (`text-display`: mono 600, 2.25rem, line-height 1, tracking -0.03em): the size of the answer. The three severity totals of the answer strip and the cluster's "current / maximum" nodes in the growth pair, where the slash is 400 in Muted Ink.
- **Headline** (600, 1.875rem, tracking tight): the page title of every signed-in screen. The sign-in card title steps down to 1.5rem.
- **Title** (600, 1.125rem): section headings inside a page ("Where room runs short", the action plan, the findings of a row), dialog titles, and the name of a finding in its detail. The brand word is 1.125rem 700, tracking tight. Claim headings and the severity groups of the findings list are 1rem 600.
- **Body** (400, 0.875rem / 1.25rem): sentences, table cells, controls, captions. A finding's sentence in its detail is 1rem. Small meta under a name is 0.75rem.
- **Data** (mono 400, 0.875rem): every cluster name, number, version, user name, time zone, and count. A count row sets it at 1.25rem 600.
- **Evidence lead** (mono 600, 1.125rem): the leading evidence value of a proof, in Ink beside its muted name; the rest of the evidence follows in mono at body size.
- **Label** (`label` utility: 600, 0.6875rem, line-height 1rem, tracking 0.08em, uppercase, Muted Ink): table headers and the detail headings of a finding (Observed, What to do, Evidence). A sort button inside a header takes it whole.

### Named Rules
**The Mono Names Rule.** Every name and number a cluster produced is set in JetBrains Mono, and only those. Interface words never are: a placeholder such as "Unavailable", the unknown node pool, a workload kind, a scope type, the word "Version" beside its number.

**The Upright Rule.** No italic, in either face. A secondary number is told apart by size and Muted Ink, never by slant.

**The Clear Underline Rule.** Every underline sits 0.22em below the baseline, set once on the root, because Mona Sans draws its own on the baseline, where skipping the ink of the letters cuts it into dashes. A link that names its own offset (`underline-offset-4`) keeps it.

**The One Label Rule.** Uppercase tracked text has exactly one size, the `label` utility. It names a column or a section; it never sits as a decorative tag above a title.

**The One Dot Rule.** A line of three or more items parts them by a gap, through `Separator`: 1rem and a space where the line may wrap, the middle dot kept for screen readers alone.

## Layout

The signed-in screen is a two-column grid from `md` (768px): a side column of 14rem (16rem from `xl`, 1280px) on the Ground, closed by a Hairline at its right, and the working area. Below `md` the side column stacks on top, closed by a Hairline underneath. The working area has a header row of icon buttons at the right, closed by a Hairline, and a main region padded 24px (32px from `xl`) at the sides and 28px top and bottom. The app is built for desktop from 1024px.

Spacing follows Tailwind's 4px step; 2, 4, 6, 8, 12, 16, 20, 24, 28, and 32px are the values in use. Controls and table cells pad 12px by 8px; a pair's halves pad 16px; a finding detail 20px; a dialog 24px; the sign-in card 28px. Stacks of pairs part by 12px, overview sections by 32px. Forms keep to 48rem (`max-w-3xl`), the account page to 36rem.

A report screen stacks the title row (title, the meta line of cluster and period, the PDF action at the right), the seven tabs over a Hairline baseline, then the tab. A claim-and-proof pair splits into two equal halves from `lg` (1024px) and stacks below it. On the findings tab from `lg` the list takes up to 26rem and the detail the rest, sticking 16px under the window top and scrolling within the window height. In an opened row, a finding and its proof sit side by side from `lg`, the proof in a 20rem column.

Tables never scroll sideways. Cells wrap between words; only a mono name or a value longer than the table breaks inside it. The header sticks under the window top. An opened row reveals itself between the sticky header and the window bottom.

## Elevation & Depth

The system is flat at rest. The theme resets every Tailwind shadow scale (`--shadow-*`, `--inset-shadow-*`, `--drop-shadow-*`, `--text-shadow-*`) and defines one: `shadow-float`. Panels, tables, and pairs separate from the Ground by a Hairline frame and the Panel tone alone. Floating layers sit at z-50 above the sticky table header.

### Shadow Vocabulary
- **Float** (`box-shadow: 0 10px 28px -8px rgb(13 14 16 / 0.16), 0 2px 6px -2px rgb(13 14 16 / 0.08)`; dark `0 12px 32px -8px rgb(0 0 0 / 0.6), 0 2px 6px -2px rgb(0 0 0 / 0.4)`): menus, popovers, dialogs, and confirmations, over a Hairline frame. The tooltip takes none: it inverts to an Ink ground instead.

### Named Rules
**The One Elevation Rule.** Only a floating layer casts a shadow, and only `shadow-float`. Nothing at rest on the page is lifted.

## Shapes

Two radii, from Tailwind's own scale: controls take 6px (`rounded-md`: buttons, inputs, selects, alerts, navigation entries, menu items, tooltips, the segmented track), panels and floating layers take 8px (`rounded-lg`: tables, claim-and-proof pairs, the finding detail, the sign-in card, dialogs, popovers, menus). The checked segment takes 5px so it nests inside the track's 2px inset. Borders are 1px Hairline; the active tab rule is 2px Ink; the focus ring is a 2px accent outline with a 2px offset (inset on menu items and list entries).

Status shapes are 12px SVGs (16px in the answer strip, 10px on the data quality tab): a filled diamond (critical), a filled triangle (attention), a filled circle (healthy), an open circle (undefined), and a dash (unavailable). The brand mark is the load-line disc: a 2.25px Ink circle crossed at its centre by a line in the accent. The browser-tab icon (`web/public/favicon.svg`) is the same drawing with heavier strokes, 3 of 30 for the circle and 3.25 for the line, so that 16px stays dark; its Ink and accent follow the system theme.

## Components

### Buttons
- **Shape:** gently rounded (6px), 1px border, min height 36px, 12px horizontal padding, 0.875rem weight 500.
- **Primary:** Signal Blue fill and border with its foreground; Signal Blue Hover under the pointer.
- **Secondary (default):** Panel fill, Input Border stroke, Ink; Hover under the pointer.
- **Danger:** Critical fill and border with Panel text, its hover twin under the pointer; only on the confirm button of a destructive dialog. The action that opens that dialog is secondary.
- **Hover / Press:** the hand cursor on every enabled button; the hover colour changes at once; a press sinks the button 1px, which glides over 100ms only under `motion-safe:`.
- **Focus / Disabled:** 2px accent outline offset 2px; disabled drops to 60% opacity with a not-allowed cursor and takes neither hover nor press.
- **Icon button:** a secondary button 36px square with a 20px Phosphor icon; its name is in `aria-label` and in a tooltip on hover and focus, drawn inside the landmark that holds the button.

### Segmented control
A grey track (Hover) with a 2px inset and 6px corners; each option is muted text at 0.875rem 500, 32px high. The checked option is a raised chip: Panel fill, Input Border edge, Ink text. A released option turns Ink under the pointer; focus outlines the option.

### Severity toggles
The findings filter: one 6px-cornered toggle per severity, its status mark then its count in mono. Pressed: an Ink outline on Panel, the shape in its status colour. Released: a Hairline outline on no fill, Muted Ink, and a plain shape in the text colour (the shape and the word still name the status). Under the pointer both take Hover, and a press sinks 1px as a button does.

### Inputs / Fields
- **Style:** Panel fill, 1px Input Border, 6px radius, min height 36px, 12px padding. The label sits above at 0.875rem 500; a hint below in Muted Ink; an error below in Critical.
- **Select:** the browser's own `<select>`, styled like a field with its native arrow removed and a 16px Phosphor caret drawn in Muted Ink at the right.
- **Focus:** 2px accent outline, 2px offset.
- **Checkbox:** the native box tinted with Signal Blue.
- **Period picker:** a choice is a 36px row that takes Hover under the pointer and the Signal Blue fill when chosen; the calendar's days are mono, the chosen ends filled Signal Blue and the span between them on Info Surface.

### Navigation
- **Side column:** brand at the top, entries at 0.875rem 500 in Muted Ink with 8px by 12px padding and 6px corners. The current page takes the Current tint and Ink; another entry under the pointer takes Hover and Ink. The user name sits at the foot in mono, and the version as the word upright beside its number in mono.
- **Report tabs:** 0.875rem 500 tabs over a Hairline baseline; the active tab is Ink with a 2px Ink underline, the rest Muted Ink; an inactive tab under the pointer turns Ink over a 2px Input Border underline. The data quality tab carries a 10px attention shape and the count in mono when the report has warnings.

### Claim-and-proof pair (signature)
A Panel with a Hairline frame and 8px corners, the claim above its proof, parted by a Hairline. It holds the cluster's growth: the worst cluster finding and the autoscaler's facts as the claim; the caption, the nodes at display size, and the nodes' proof bar as the proof. With no proof the claim stands alone.

### Findings chart
Two Panels side by side from `lg` (20rem and the rest), on the overview and above the filters of the Findings tab. The donut is an SVG ring in the status fills, its slices parted by a 2px stroke in Surface, with the total in mono at display size and the word "findings" at its centre, and the three counts beneath it, each beside its shape. The scope panel holds a caption and one row for each scope the view gives, worst first: the scope name (a status mark on the overview), a 16px bar on a Track, and the three counts in mono beside their shapes. Every bar is drawn on one scale, the largest row total. On the overview the worst finding's sentence is a link under its bar; on the Findings tab the name is plain and each segment is a link that sets the scope and severity of the list. A report with no finding draws neither and says "No finding".

### Proof bar (signature)
A value drawn against its reference on one track, as SVG whose lengths are attributes, never styles.
- **Track:** an 8px Track band across the full width.
- **Fill:** full track height from the start (zero, or the low end of a range) up to the reference.
- **Overrun:** past the reference, the fill runs on at half height.
- **Reference tick:** a 2px Strong Line tick where the reference falls, drawn once the value reaches it.
- **Ceiling:** the scale holds at most 4 times the reference; a value beyond ends in a break of two Panel slashes near the end, and the printed numbers carry the size of the overrun.
- **Units:** a count up to 40 is divided per unit by 2px Panel gaps.
- **Floor:** the minimum of nodes is a dashed 1.5px Strong Line tick.
- **Colour:** the finding's severity (critical, attention, healthy), or Strong Line ink when the view gives none.
- **Evidence above:** the leading value at 1.125rem mono 600 beside its muted name, the rest after it at body size.
- **Motion:** the fill grows once from the left (`animate-proof`) only under `motion-safe:`. The drawing is hidden from assistive technology; the numbers beside it carry the meaning.

### Answer strip
The first row of the overview: the three severity totals, each a 16px shape, the count at display size in mono, and its word in Muted Ink at 0.875rem, parted by 32px. Each links to the findings of its severity unless the view's minimum leaves it out, and a "See all findings" accent link follows. When the data has warnings, a notice sits at its right from `xl`: an attention shape, the tab's name in 600 with the count in mono, up to three warnings in Muted Ink, the rest as a count, and a link.

### Data table
The one table of the app: a Panel with a 1px Hairline frame and 8px corners, labels in the sticky header, Hairline rules between rows, a 48px expander column of transparent icon buttons. An opened row is tinted Info Surface; its detail row sits on Ground at 24px by 16px padding and holds the row's findings, each a status mark and sentence beside its proof bar. A peak cell stacks the peak in mono over its share in mono 0.75rem Muted Ink, each with its status shape, and takes the critical or attention surface by the view's status. A row with no finding reads "No finding" in Muted Ink with no mark.

### Findings list and detail
The list groups findings by severity under 1rem 600 headings with their counts in mono, each group a Panel with Hairline rows. An entry shows its status shape, scope type muted, the name in mono on one line, and the sentence clamped at two lines; the selected entry is tinted Info Surface. The detail is a Panel with 20px padding and 8px corners: the name at 1.125rem, then Observed, What to do, and Evidence under labels, the evidence led by its proof bar.

### Status mark
Shape, then label, then colour. The 12px shape carries the status hue; the word keeps the surrounding text colour and is semibold for critical. Where the label is not the status word, the status word is added for screen readers.

### Alerts
6px corners, 1px border, 8px by 12px padding, 0.875rem. Error: Critical Surface, Critical Border, Critical Ink. Notice: Info Surface, Hairline border, Info Ink.

### Floating layers
Menu, popover, dialog, and confirmation are Panel with a 1px Hairline frame, 8px corners, and `shadow-float`; a dialog pads 24px and caps at 36rem (28rem for a confirmation) over the Scrim. A highlighted menu item takes the Current tint; the item reached by the keyboard also takes an inset accent outline. The tooltip inverts: Ink ground, Ground text, 0.75rem, 6px corners, no shadow.

### Motion
Motion appears only under `motion-safe:` or `prefers-reduced-motion: no-preference`, and moves only `transform` and `opacity`.
- **Layers** (`layer-motion`): a menu, popover, or tooltip fades in over 140ms (cubic-bezier(0.16, 1, 0.3, 1)), growing from 97% out of the point Radix names and 4px off its side, and fades out over 90ms.
- **Modals** (`scrim-motion`, `dialog-motion`): the scrim fades in over 150ms; the dialog fades and settles from 98% over 160ms; both leave over 100ms.
- **Proof** (`animate-proof`): a bar's fill scales in from the left once, 700ms, cubic-bezier(0.16, 1, 0.3, 1).
- **Press:** a button and a severity toggle sink 1px over 100ms ease-out.
- **Work in progress:** the current step of the progress page and the accent ring beside "updating" on the reports list pulse.

## Do's and Don'ts

### Do:
- **Do** take every colour from a token in `web/src/theme/theme.css` and add each new foreground and background pair to `web/src/theme/pairs.ts` before shipping it.
- **Do** draw a status as its shape first, then its word, then its colour, through `StatusMark` or `StatusShape`.
- **Do** set a finding's proof beside its claim: the sentence, then the evidence and its proof bar, drawn from the numbers the view gives.
- **Do** colour a proof bar by its finding's severity, or in ink when the view gives none.
- **Do** set every cluster name and number in JetBrains Mono and every interface word in Mona Sans, upright.
- **Do** keep controls at 6px corners and panels and layers at 8px, framed by a Hairline.
- **Do** reserve Signal Blue for the primary action, links, focus, and the chosen or current value.
- **Do** put motion only under `motion-safe:` and keep it to layers opening, a proof filling once, a press, and work in progress.
- **Do** style through classes and the theme's utilities, and give a bar its length as an SVG attribute; the CSP allows no inline style and no injected style element.

### Don't:
- **Don't** colour a word with a status hue; the colour belongs to the shape and the bar.
- **Don't** cast a shadow from anything but a floating layer, and never one other than `shadow-float`; no glow, no gradient.
- **Don't** tint a neutral: every grey keeps its channels within 3 units.
- **Don't** use blue for a status or for decoration.
- **Don't** compute a threshold in the web app to decide a tint, a bar's colour, or a status.
- **Don't** set anything in italic.
- **Don't** set uppercase text at any size other than the `label` utility, or use it as a tag above a title.
- **Don't** load a font, a stylesheet, or an icon from outside the bundle.

## The PDF

The report on paper keeps its own look: the instrument drawn in ink, with the status colours of the web and no other colour. Every fill, rule, and glyph is `black`, `white`, a `luma(...)` grey, or a colour of `template/capacity/palette.typ`, whose six values equal the light tokens of the web theme (a test holds them equal): critical `#be1e45`, attention `#8f5b00`, healthy `#24774a`, neutral `#5c5c5e`, critical surface `#fdedf0`, attention surface `#fdf4e3`. Words and numbers stay in ink. It shares JetBrains Mono and the load-line disc with the web app, so a name or a number reads the same on both; its words stay in Public Sans.

### Faces and sizes
- **Words:** Public Sans (Regular, SemiBold, Bold, and their italics) at 9pt on the verdict page and 7pt to 8.5pt in tables. Title 18pt Bold; the plan heading 11pt SemiBold; the totals line 13pt.
- **Names and numbers:** JetBrains Mono. A cluster name, a period, an issue time, an identifier (at 0.92em, never hyphenated), and every number cell. The verdict numeral "current / maximum" is mono 30pt SemiBold, tracking -0.04em; the gauge scale is mono italic 5.5pt.
- **Labels:** the eyebrow over a block is 7pt SemiBold, tracking 0.08em, upper case, in Muted grey.
- **Table text:** per layout, 6.8pt to 8.5pt, with a row inset of 2pt or 3.5pt. The page margins are one inch at the top and the sides and half an inch at the bottom.

### Greys
Foreground is black. Muted text (the eyebrow, the footer, the facts under the gauge, the issue block) is `luma(35%)`. The words of a healthy, unavailable, or no-base mark are `luma(45%)`. The track of a bar is `luma(93%)`. Hairlines between rows and the colophon rule are `luma(78%)`. A gauge with no hazard has bands tinted `luma(93%)`, `luma(85%)`, and `luma(74%)`, with a `luma(55%)` band edge.

### Status glyphs
Status is a shape, then a word. The five glyphs (the diamond of critical, the triangle of attention, the circle of healthy, the empty circle of no base, the dash of unavailable) are set in JetBrains Mono, the one face that holds all five. Critical is bold. A glyph takes the colour of its status (critical, attention, healthy; neutral for no base and unavailable); the word beside it keeps its ink. A glyph never stands without its word where the page has room for the word.

### The gauge
The depth gauge is drawn in black ink. Up to sixty nodes, one band per node, filled bands in the colour of the cluster finding's hazard (critical or attention), lightened by thirds of the scale, or in greys when there is no hazard; above sixty, three bands over the thirds of the scale, each filled to the share of the current count that falls in it. A dashed 0.7pt line marks the minimum, rounded to a whole node. Once the cluster is at its maximum and the report gives it a finding, the last band is hatched: diagonal strokes of 1.1pt (white over a status colour, black over grey) over the darkest band. The scale beneath shows every node up to ten, else ten values spread evenly, the first and the last included.

### The findings donut
A ring 140pt wide of the findings by status, drawn from the counts of the totals: one slice per status that has a count, clockwise from the top (critical, attention, healthy), parted by a 2pt gap, in the status colours. The total sits at the centre in mono Bold 26pt with the word "findings" under it in Muted 8pt, and a legend follows: a swatch, the glyph, the count in mono, the word. A single status is a full ring. A report with no finding draws no ring; the totals strip says so and the cluster block stands alone.

### The verdict page
The first page of a report the command writes. From the top: the title band, the severity totals with the leading count in bold, the data-quality line, then two columns (0.9fr, 1.5fr). The left column opens with the findings donut and then holds the cluster autoscaling block, the numeral, the gauge, the caption, the worst cluster finding, the node-evidence warning, and the facts. The right column is the pressure by scope, one row after a hairline, worst scope first: the status glyph and the scope, a bar of its critical, attention, and healthy findings against its scale over a `luma(93%)` track, the sentence, and the counts as glyphs and numbers. Below both, the prioritized action plan with its footer line ("5 of 10 actions"), and the legend of the five statuses.

### The title band
One block at the head of the first page: the title at 18pt Bold and, 10pt under it, the cluster in mono and the period with its numbers in mono, each kept whole, so a long cluster name moves the period to the next line instead of breaking a word; the profile, the author, and the issue time at 8pt Muted, right aligned, the last on the baseline of the period. The rule below is graphite's own title rule at 0.8pt.

### Tables
Each table layout names a face for every column. A `num` column is set in mono and aligned right; a `name`, `text`, or `mark` column is aligned from the left. Cells sit at the middle of the row. The header is SemiBold, repeats at a page break, and is closed by a 0.8pt black rule; every other row is parted by a 0.5pt `luma(78%)` hairline. A number cell that holds a critical or attention status mark is tinted with the critical or attention surface (the worse of the two); no other cell has a fill.

### The colophon
The last block of the document: a hairline rule, then the load-line disc (a ring crossed by a line, drawn in black at 8pt) and the line naming the program, CapacityRoom, at 7pt Muted.
