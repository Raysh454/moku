# Frontend design rules

Explicit constraints so the UI stays cohesive and doesn't drift into generic
"AI slop". Follow these when adding or restyling UI.

## Containers / cards
- **Whitespace is the default separator.** Group related content with spacing
  and a heading, not a bordered/filled box. Borders don't scale with the
  viewport; spacing does.
- **One elevation level.** Never nest a bordered/filled card inside another
  (no card-in-card). If an outer region already reads as a group, inner
  regions get spacing or a hairline, not their own border.
- **A border/fill must earn its place.** Use one only for genuinely distinct
  surfaces: an actual editor/diff/iframe pane, a popover/modal, the sidebar.
  Lists of stats, metrics, evidence, or grouped changes are *not* cards.
- **Dividers sparingly.** Prefer space. A hairline (`border-border/60`) is for
  separating a header from a body or two dense sections — not every row, and
  never when there's little to separate.

## Hierarchy
- Establish grouping with **type scale + weight + color**, not boxes:
  `SectionHeading` for titles, `text-helper`/`text-muted` for secondary text,
  tabular-nums for figures.
- Avoid the generic "badge + headline + N-card grid" dashboard pattern.

## Tokens
- Use the `@theme` tokens only (`bg`, `card`, `surface`, `accent`, `success`,
  `danger`, `warning`, `border`, `primary`, `helper`, `muted`). No new ad-hoc
  colors, no hardcoded hexes in components, no purple-on-white gradients.
- Compose with the `components/ui` primitives; don't re-derive button/label
  styling inline.
