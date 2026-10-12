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
  muted-ink: "#455560"
  hairline: "#cfdbe2"
  neatline: "#22323d"
  input-border: "#6c808d"
  sounding-blue: "#14375c"
  action-ink: "#22323d"
  action-ink-fg: "#ffffff"
  sea-green: "#155e46"
  ochre: "#784800"
  chart-magenta: "#a0135a"
  slate-neutral: "#445661"
  hazard-surface: "#fbe8f1"
  hazard-border: "#e9b1cd"
  hazard-surface-ink: "#76104a"
  info-surface: "#e3f0f7"
  info-surface-ink: "#1b4870"
  depth-1: "#c5deea"
  depth-2: "#aacee0"
  depth-3: "#8bbcd5"
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
  sounding:
    fontFamily: "JetBrains Mono Variable, ui-monospace, monospace"
    fontSize: "0.6875rem"
    fontStyle: "italic"
    fontWeight: 400
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
  severity-toggle-pressed:
    backgroundColor: "{colors.action-ink}"
    textColor: "{colors.action-ink-fg}"
    rounded: "{rounded.hairline}"
    padding: "0 12px"
    height: "36px"
  severity-toggle:
    backgroundColor: "{colors.sheet}"
    textColor: "{colors.sheet-ink}"
    rounded: "{rounded.hairline}"
    padding: "0 12px"
    height: "36px"
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

The system is flat, dense, and drawn with hairlines. Every radius is 2px and nothing casts a shadow. Structure comes from neat-lines, hairline rules, and tonal tints. Names and numbers are set in JetBrains Mono, and its italic is the soundings voice of secondary numbers. Interface text is set in Public Sans. The signature drawing is the depth gauge: the cluster's nodes as a sounded depth scale, whose last band is hatched once no room is left and the report gives the cluster a finding.

The visual system answers to two binding project rules. Status is a shape, then a label, then a colour, so no status is ever carried by colour alone. Every foreground and background drawn together is a listed token pair that passes a contrast test in both themes.

**Key Characteristics:**
- White chart ground, pale shallows shell, dark neat-lines.
- Hairline shapes: 2px radius everywhere, no shadows anywhere.
- Three depth tints that follow the status the report gives a value, never a threshold of the web app's own.
- Hatching is the one hazard device, and it follows a status of the report: magenta for critical, ochre for attention on the gauge.
- Monospace for every name and number, and only for them; mono italic for secondary soundings.
- One uppercase tracked label size for table headers and small section headings.
- Motion only under reduced-motion-safe conditions: the gauge sounds once, and a pulse marks work in progress.

## Colors

A cool chart palette: blue-cast neutrals, three blue depth tints, and three chart status hues. Each token has a light value (frontmatter) and a night-chart value (sidecar), switched by `data-theme="dark"` on the root.

### Primary
- **Sounding Blue** (accent): links, the active tab rule, the focus ring, the current-level line of the depth gauge, and the load-line stroke of the brand disc. It is the chart's own ink for navigation, not a fill.
- **Action Ink** (action / action-fg): the filled primary button, the checked segment, and the native control accent. In light it is the neat-line ink; in dark it inverts to a pale slate with dark text.

### Secondary
- **Chart Magenta** (critical): the critical status, field error text, the danger button, and the hazard hatching. Critical labels are set bold.
- **Ochre** (attention): the attention status, and the gauge hatch when the cluster finding is attention.
- **Sea Green** (healthy): the healthy status.
- **Slate Neutral** (neutral): the undefined and unavailable statuses.

### Tertiary
- **Depth 1 / Depth 2 / Depth 3** (depth-1..3): graded water. Depth 1 tints a peak cell in attention and is the ground of the hazard hatch on a critical peak cell. Depth 2 and Depth 3 lie only under the gauge and carry no text. The gauge tints its filled bands from Depth 1 to Depth 3 toward the maximum; those tints mark position on its own scale, not a status. Night values: #17324a, #1b3f5e, #1f4b70. Each step stands at least about 1.2:1 from the next, and Depth 1 as far from Sheet and from Info Surface, so a tint stays visible on an opened row in both themes.

### Neutral
- **Chart Ground** (bg): page background.
- **Chart Ink** (fg): body text; also the tooltip ground.
- **Sheet** (surface / surface-fg): tables, inputs, secondary buttons, dialogs, popovers, menus.
- **Shallows** (sidebar / sidebar-fg): the side shell and the sign-in header.
- **Shallows Active** (sidebar-active / sidebar-active-fg): the current navigation entry and a highlighted menu item.
- **Muted Ink** (muted-fg): captions, meta lines, labels, secondary values.
- **Hairline** (border): row rules, table frames, the tab baseline.
- **Neatline** (neatline): the shell's closing lines, the baseline of the report tabs, the frame of every floating layer, the double frame of the sign-in card, the outline of a released severity toggle, and the gauge outline and ticks.
- **Input Border** (input-border): field and secondary-button strokes, held at 3:1.
- **Scrim** (`rgb(14 26 34 / 0.45)` light, `rgb(0 0 0 / 0.65)` dark): the overlay behind a modal.
- **Hazard Surface / Border / Ink** and **Info Surface / Ink**: the error alert and the notice alert. Info Surface is also the ground of an opened table row; its night value is #122332, kept apart from the night depth tints.

### Named Rules
**The Token Pair Rule.** A colour is a token pair. Every foreground drawn on a background is listed in `web/src/theme/pairs.ts` and must reach 4.5:1 for text, or 3:1 for a large heading, an input border, a focus ring, or a line, in both themes. A new pair on screen enters the list first.

**The Status-Led Depth Rule.** A depth tint follows the statuses the report view gives a value. A peak cell takes the worse of the status of its peak and the status of its share of the request: attention takes Depth 1, critical takes the hazard hatch over Depth 1, and every other status takes none. The hazard of the gauge follows the worst cluster finding. The web app never computes a threshold of its own. The one exception is the State column of the nodes, which marks Not ready as attention and Unknown as unavailable though the view gives no status for readiness: an owner decision of 2026-10-11 (option 1A).

**The One Hazard Rule.** Hatching marks a status of the report and nothing else. In a cell it is magenta: 135 degrees, 1.5px lines every 6px, in the `--hazard-stripe` token, the critical colour laid over Depth 1 at 10% in light and 12% in dark, so every text on the cell reaches 4.5:1 on the stripe as well as on the ground. On the gauge, the last band of a full scale takes 2.2px lines over Depth 3 in the colour of the worst cluster finding: magenta for critical, ochre for attention. With no such finding a full scale is plain Depth 3, closed by a Neatline; the numbers alone never hatch it.

## Typography

**Display Font:** JetBrains Mono Variable (with ui-monospace, monospace)
**Body Font:** Public Sans Variable (with system-ui, sans-serif)
**Label/Mono Font:** JetBrains Mono Variable for data; Public Sans for labels

**Character:** Public Sans stands upright for names of things in the interface: navigation, headings, sentences. JetBrains Mono carries every name and number of the cluster, and its italic speaks the secondary soundings: shares, counts, gauge scale numbers. Both faces are self-hosted from npm packages, because the CSP allows no outside source.

### Hierarchy
- **Display** (`text-display`: mono 600, 3.5rem, line-height 1, tracking -0.025em): the verdict numeral "current / maximum" of the overview. One per screen; the slash between is set in Public Sans 400 in Muted Ink.
- **Headline** (600, 1.25rem): the page title of every screen and the sign-in card.
- **Title** (600, 1.125rem): dialog titles and the brand word (700, tracking tight). Section headings inside a page step down to 1rem 600, the severity groups of the findings list among them; the title of a finding detail stays at 1.125rem.
- **Body** (400, 0.875rem / 1.25rem): sentences, table cells, controls, captions. Small meta under a name is 0.75rem.
- **Data** (mono 400, 0.875rem): every cluster name, number, version, user name, and time zone. Count rows set it at 1.25rem 600.
- **Sounding** (mono italic): the share under a peak, every count after its status word, the minimum-node and event figures, and the gauge scale (`text-sounding`, 0.6875rem). A count always follows its word: "Critical 9".
- **Label** (600, 0.6875rem, line-height 1rem, tracking 0.08em, uppercase, Muted Ink): table headers and small section headings, among them the detail headings of a finding (Observed, What to do, Evidence). A sort button inside a header takes it whole.

### Named Rules
**The Mono Names Rule.** Every name and number a cluster produced is set in JetBrains Mono, and only those. Interface words never are: a placeholder such as "Unavailable", the unknown node pool, a workload kind, a profile's display name, the word "Version" beside its number. A summary line mixes the two, its values in mono and its words upright.

**The One Label Rule.** Uppercase tracked text has exactly one size (0.6875rem). It names a column or a section; it is never a decorative tag above a title.

## Layout

The signed-in screen is a two-column grid at `md` (768px) and above: a 16rem shallows column on the left, the working area on the right. Below `md` the shell stacks on top, closed by a neat-line underneath instead of at its side. The working area has a header row of icon buttons at the right, closed by a neat-line, and a main region with 24px padding.

Spacing follows a 4px step: 2, 4, 6, 8, 12, 16, 24, and 40px are the values in use. Controls and table cells use 12px horizontal and 8px vertical padding; stacks of sections use 16px gaps; the overview verdict uses 24px vertical padding and a 40px column gap. At `lg` (1024px) the verdict splits 4fr to 7fr: growth on the left, pressure by scope on the right.

Tables never scroll sideways. Cells wrap between words and never inside one; only a mono name may break anywhere, and a status word never breaks. The header sticks under the window top. An opened row reveals itself between the sticky header and the window bottom, and only a value longer than the table breaks inside it. Report screens have seven tabs under the title, over a Neatline baseline that also closes the top of the overview verdict; the PDF action sits at the right end of the title row. On the findings tab at `lg` and above, the detail of the selected finding sticks 16px under the window top and scrolls within the window height.

## Elevation & Depth

The system is flat. The theme resets every Tailwind shadow scale (`--shadow-*`, `--inset-shadow-*`, `--drop-shadow-*`, `--text-shadow-*`) to nothing. Depth is conveyed by three means only: neat-line frames, tonal tints (shallows, info surface, the three depth tints), and stacking order. Floating layers (menu, popover, tooltip, dialog, alert dialog) sit at z-50 above the sticky table header, framed by a Neatline border on Sheet; modals add the Scrim.

### Named Rules
**The No-Shadow Rule.** Nothing casts a shadow. A layer separates from the page by its neat-line frame alone.

## Shapes

Every radius is 2px: a near-square corner that keeps a hairline drawing from looking cut. Borders are 1px; the active tab rule is 2px; the focus ring is a 2px Sounding Blue outline with a 2px offset. The active navigation entry gets an inset 1px Neatline ring. The sign-in card has a double neat-line frame: a 1px Neatline border and a 1px Hairline outline 4px outside it. The brand mark is a load-line (Plimsoll) disc: a 2.25px circle crossed at the centre by a Sounding Blue line. Status shapes are drawn as 12px SVGs: a filled circle (healthy), a filled triangle (attention), a filled diamond (critical), an open circle (undefined), and a dash (unavailable).

## Components

### Buttons
- **Shape:** gently squared (2px), 1px border, min height 36px, 12px horizontal padding, 0.875rem weight 500.
- **Primary:** Action Ink fill and border with Action Ink foreground.
- **Secondary (default):** Sheet fill, Input Border stroke, Sheet ink.
- **Danger:** Chart Magenta fill and border, Sheet text; only on the confirm button of a destructive dialog. The action that opens that dialog is a secondary button.
- **Focus / Disabled:** 2px Sounding Blue outline offset 2px; disabled drops to 60% opacity with a not-allowed cursor. No hover colour shift.
- **Icon button:** a secondary button 36px square with a 20px Phosphor icon; its name is in `aria-label` and in a tooltip on hover and focus.

### Segmented control
A row of radio labels joined edge to edge (-1px overlap), 2px outer corners, 16px padding, under a legend in the field-label style (0.875rem 500). The checked segment takes the Action Ink fill; focus outlines the segment.

### Severity toggles
The findings filter: one 2px-cornered toggle per severity, the status mark then its count in mono italic. A pressed toggle takes the checked-segment style, Action Ink fill and Action Ink Foreground text, and its mark takes that text colour while keeping its shape and word; a released toggle is Sheet with a Neatline outline and the mark in its status colour. When the filters leave no finding, a secondary button "Show all findings" clears the severities, the scope, and the search.

### Inputs / Fields
- **Style:** Sheet fill, 1px Input Border, 2px radius, min height 36px, 12px padding. The label sits above at 0.875rem 500; a hint below in Muted Ink; an error below in Chart Magenta. A select is the browser's own `<select>`, styled the same.
- **Focus:** 2px Sounding Blue outline, 2px offset.
- **Period (New report form):** a field of its own above the report language, at most 24rem wide and full width below that; its trigger is an input-style button naming the period. The popover opens below the trigger at up to 46rem (the window less 2rem when narrower), Sheet with the Neatline frame: the length and the typed range on the left, the quick lengths and the calendar periods side by side in two columns of a 24rem pane on the right, each choice on one line in both languages, with no divider between them. A profile that limits its periods gets a single column of those periods in a 36rem popover. The Options box under the form holds the time zone, the report profile, and the minimum severity only; its closed summary lists those three.
- **Checkbox:** native box tinted with Action Ink.

### Navigation
- **Shell:** Shallows column, brand at the top, links at 0.875rem with 8px by 12px padding. The current page takes the Shallows Active tint, weight 600, and an inset Neatline frame; Reports stays current on a report and on a job in progress. The user name sits at the foot in mono, and the version as the word "Version" upright beside its number in mono.
- **Report tabs:** a row of 0.875rem tabs, 10px apart in padding, over a Neatline baseline; the active tab is Chart Ink 600 with a 2px Sounding Blue underline, the rest Muted Ink. When the report has data-quality warnings, the Data quality tab carries a 10px attention shape and the count in mono italic after its word, and its accessible name states the count. The seven tabs hold one line at 1024px in both languages.

### Data table
The one table of the app: a Sheet container with a 1px Hairline frame and 2px corners, uppercase labels in the sticky header, Hairline rules between rows, an expander column of 48px. An opened row is tinted Info Surface and its detail row sits on Chart Ground at 24px by 16px padding. A peak cell stacks the peak in mono over its share in mono italic, and takes the depth tint its statuses earn. A peak with no number shows its placeholder word alone, upright in Muted Ink, with no share under it. On the nodes tab and the HPA table the Status cell carries a line under the severity, in Sheet ink at the small size on a 14rem measure, that says why: a node's reason, an HPA's worst finding sentence. A row with no finding has none. The node pools table has a Status column after the pool's name, drawn the same way from the pool's worst finding, with no line under it. In the findings list each entry's sentence clamps at two lines, so the key words of a long sentence stay; the name above it stays on one.

### Status mark
Shape, then label, then colour. The 12px shape and the word share the status hue; critical words are bold. Where the label is not the status word, the status word is added for screen readers.

### Alerts
2px corners, 1px border, 8px by 12px padding, 0.875rem. Error: Hazard Surface, Hazard Border, Hazard Ink. Notice: Info Surface, Hairline border, Info Ink.

### Floating layers
Menu, popover, dialog, and alert dialog are Sheet with a 1px Neatline frame and 2px corners; a dialog pads 24px and caps at 36rem wide (28rem for a confirmation). A highlighted menu item takes Shallows Active with an inset Sounding Blue outline. The tooltip inverts: Chart Ink ground, Chart Ground text, 0.75rem.

### Depth gauge (signature)
The cluster's nodes drawn as a sounded depth scale 360px wide: one band per node up to the autoscaler maximum, inside a 1px Neatline frame; above 60 nodes the level is one band in the tints of its thirds. Counts are whole nodes: the level is rounded and held between none and the maximum. Filled bands are tinted Depth 1 to Depth 3 toward the maximum and parted by hairlines in the ground colour. A 2px Sounding Blue line marks the current level; the minimum is a dashed Neatline contour. Once no room is left, a 2px line closes the scale, and the last band and that line take the severity of the worst cluster finding: hatched and drawn in Chart Magenta for critical, in Ochre for attention; with no cluster finding above healthy, the band stays plain Depth 3 and the line is Neatline. Ticks below carry mono italic soundings (`text-sounding`), at most ten. The drawing is hidden from assistive technology because the numbers stand in the headline above it.

### Answer strip
The first thing under the report tabs, 8px lower than the tab stack's gap: one row at 1.25rem of the three severity totals, each a 16px shape, its word, and its count in mono italic, parted by a Muted Ink middle dot. The worst severity present is drawn in its status colour and weight 600; the others in Chart Ink. Each total links to the findings of its severity unless the view's minimum severity leaves it out, and a 0.875rem "See all findings" link follows; the totals wrap as whole items. A report with no finding says so in the strip instead. Under it, only when the data has warnings, a notice: an attention shape, the label "Data quality" in 600 with the count in mono italic, up to three warnings one per line, "+N" in Muted Ink for the rest, and a link to the data quality tab.

### Overview verdict
A section under the answer strip, closed by a Hairline bottom rule. Left: a label, the display numeral, the depth gauge, a caption, the cluster verdict (the worst cluster finding, under it the node-evidence warning when the report has it, in the attention shape and colour; the same verdict also heads the Autoscaling tab, under its title row), mono italic secondary figures, and a link. The link to the autoscaling tab sits at the start of its line, so its focus ring hugs the text. Right: pressure by scope, one row per scope type, worst severity first and the fixed order among equals, with a Hairline top rule, the scope label as a status mark in an 8rem column, then the leading finding as an underlined link, its name and peak in mono, and its severity counts, which count rows as the tabs tell them apart (a workload by its kind too).

### Action plan
The overview's table of recommendations. The Scope cell reads the scope type as a muted word in the sans face, then the full name of the scope in mono (a workload as namespace/name), wrapping rather than scrolling. An action whose finding the view holds (the same code and scope type, on the first scope) is a link to that finding, underlined in Hairline and in Sounding Blue on hover, like a pressure line; an action with no match stays plain text.

### PDF block
In the PDF popover and on the progress page: one row per language at 0.875rem, parted by Hairline dividers inside the layer's Neatline frame, with no box of their own. The rows share three columns, the language, the status, and the action, so they line up from row to row, and the action never wraps under the status.

### Reports list
The Profile cell of a row carries a second line under the profile name, in Muted Ink at the small size: the namespaces in mono (the first three, then a +N count of the rest), a middle dot, and the period in words. Cells wrap, so the table fits 1024px.

## Motion
Motion appears only under `motion-safe:`; under reduced motion everything is drawn at once and nothing pulses.
- **Sounding:** the gauge level rises once from the left (`sounding`, 600ms, cubic-bezier(0.16, 1, 0.3, 1)), then the hatched band and the closing line fade in (`hazard-in`, 200ms after 600ms).
- **Updating ring:** while a report on the list is queued, running, or has a pending PDF, a 10px Sounding Blue ring (an outline circle, not a filled dot) pulses beside the words that say the list is updating.
- **Failed report:** below the failure alert sit Back to reports, then the next step; on a failure of the profile test the administrator's primary action is Check the profile and Request again turns secondary. A failed report offers Delete report as a collected one does.
- **Progress step:** on the progress page the current step pulses.
- Navigation links have no transition.

## Do's and Don'ts

### Do:
- **Do** take every colour from a token in `web/src/theme/theme.css` and add each new foreground and background pair to `web/src/theme/pairs.ts` before shipping it.
- **Do** draw a status as its shape first, then its word, then its colour, through the status mark.
- **Do** set every cluster name and number in JetBrains Mono, and secondary numbers in its italic; set every interface word upright, placeholders included.
- **Do** keep every corner at 2px and separate layers with a Neatline frame.
- **Do** tint a value only by the statuses the report gives it: Depth 1 for attention, the hazard hatch for critical.
- **Do** put motion only under `motion-safe:` and keep it to the sounding of the gauge and the pulses of work in progress.
- **Do** style through classes and the theme's utilities; the CSP allows no inline style and no injected style element.

### Don't:
- **Don't** add a shadow, a glow, or a gradient to any surface; the theme resets every shadow scale.
- **Don't** use magenta hatching or Chart Magenta for anything but critical, and never hatch the gauge by its numbers alone.
- **Don't** let a depth tint carry Depth 3 under text; Depth 3 lies only under the gauge.
- **Don't** set a label in uppercase at any size other than 0.6875rem, or use it as a decorative tag above a title.
- **Don't** compute a threshold in the web app to decide a tint or a status.
- **Don't** load a font, a stylesheet, or an icon from outside the bundle.
- **Don't** make the dark theme glow: night-chart hues stay muted.

## The PDF

The report on paper is the same instrument drawn in ink alone. It is monochrome: every fill, rule, and glyph is `black`, `white`, or a `luma(...)` grey, and no page holds a coloured pixel. It shares the faces of the web app, so a name or a number reads the same on both.

### Faces and sizes
- **Words:** Public Sans (Regular, SemiBold, Bold, and their italics) at 9pt on the verdict page and 7pt to 8.5pt in tables. Title 18pt Bold; the plan heading 11pt SemiBold; the totals line 13pt.
- **Names and numbers:** JetBrains Mono. A cluster name, a period, an issue time, an identifier (at 0.92em, never hyphenated), and every number cell. The verdict numeral "current / maximum" is mono 30pt SemiBold, tracking -0.04em; the gauge scale is mono italic 5.5pt.
- **Labels:** the eyebrow over a block is 7pt SemiBold, tracking 0.08em, upper case, in Muted grey.
- **Table text:** per layout, 6.8pt to 8.5pt, with a row inset of 2pt or 3.5pt. The page margins are one inch at the top and the sides and half an inch at the bottom.

### Greys
Foreground is black. Muted text (the eyebrow, the footer, the facts under the gauge, the issue block) is `luma(35%)`. Healthy, unavailable, and no-base glyphs are `luma(45%)`. Hairlines between rows and the colophon rule are `luma(78%)`. The gauge bands are tinted `luma(93%)`, `luma(85%)`, and `luma(74%)`, with a `luma(55%)` band edge.

### Status glyphs
Status is a shape, then a word. The five glyphs (the diamond of critical, the triangle of attention, the circle of healthy, the empty circle of no base, the dash of unavailable) are set in JetBrains Mono, the one face that holds all five. Critical is bold and black; attention is black; the other three are `luma(45%)`. A glyph never stands without its word where the page has room for the word.

### The gauge
The depth gauge is drawn in black. Up to sixty nodes, one band per node, filled bands tinted by thirds of the scale; above sixty, three bands over the thirds of the scale, each filled to the share of the current count that falls in it. A dashed 0.7pt line marks the minimum, rounded to a whole node. Once the cluster is at its maximum and the report gives it a finding, the last band is hatched: diagonal black strokes of 1.1pt over the darkest tint. The scale beneath shows every node up to ten, else ten values spread evenly, the first and the last included.

### The verdict page
The first page of a report the command writes. From the top: the title band, the severity totals with the leading count in bold, the data-quality line, then two columns (0.9fr, 1.5fr). The left column is the cluster autoscaling block, the numeral, the gauge, the caption, the worst cluster finding, the node-evidence warning, and the facts. The right column is the pressure by scope, one row after a hairline, worst scope first. Below both, the prioritized action plan with its footer line ("5 of 10 actions"), and the legend of the five statuses.

### The title band
One block at the head of the first page: the title at 18pt Bold and, under it, the cluster in mono, the period with its numbers in mono, and the profile; the author and the issue time at 8pt Muted, right aligned, on the same baseline. The rule below is graphite's own title rule at 0.8pt.

### Tables
Each table layout names a face for every column. A `num` column is set in mono and aligned right; a `name`, `text`, or `mark` column is aligned from the left. Cells sit at the middle of the row. The header is SemiBold, repeats at a page break, and is closed by a 0.8pt black rule; every other row is parted by a 0.5pt `luma(78%)` hairline.

### The colophon
The last block of the document: a hairline rule, then the load-line disc (a ring crossed by a line, drawn in black at 8pt) and the line naming the program, k8s-capacity-report, at 7pt Muted.
