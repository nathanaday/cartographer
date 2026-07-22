# Cartographer — Slice 2.6 Design: Collapsed-Default Cards + Filter-Driven Expansion

Date: 2026-07-22
Status: Approved — implementation deferred to a future pass

## Motivation

The first deep scan (p3-edge re-scanned with inline aspects, ~120 objects with
real depth) exposed a density problem the earlier hollow store could not: at full
zoom every aspect strip auto-expands, so a 4-aspect object (sql + cpp_struct +
json + protobuf) becomes a tower of 50–60 field rows. Neighboring cards cover
each other, and the masonry packer — which reserves all-expanded heights —
produces a map that is technically correct but unreadable.

Separately, the kind filter's dimming is too subtle. Zoomed out, chips ignore the
filter entirely, so "show me only protobuf" is unanswerable at exactly the
altitude where the question matters most.

The at-a-glance skeleton (title block + strip headers with counts) is right and
stays. What changes is when detail expands and how unmistakably the filter reads.

## Scope

Frontend-only refinement (no Go/API/store changes):

1. Collapsed-by-default aspect strips at every zoom tier.
2. The kind filter becomes the expansion driver, with hard dimming of
   non-matching cards.
3. Filter awareness at the chip tier.
4. Expansion-state-aware height estimation, so the masonry packs the view that
   is actually rendered.

Out of scope: everything else (Slice 3 diff view unchanged as next major slice).

## Design

### 1. Collapsed by default

- Every aspect strip renders collapsed (header + count only) regardless of zoom.
  Zooming in never auto-expands. Full tier = title block + description + strip
  headers; field rows appear only for explicitly expanded strips.
- Expansion is explicit: the user clicks a strip header (existing toggle), or the
  kind filter expands it (below).
- A strip's expanded state resolves as: `userToggled[kind] ?? (kindFilter === kind)`
  — a manual toggle always wins over the filter default, and manual toggles are
  transient UI state (not persisted; reset on reload).

### 2. Filter-driven expansion + hard dimming

When a kind is selected in the toolbar:

- **Matching cards** (have an aspect of that kind): the matching strip
  auto-expands; other strips stay collapsed (still click-expandable).
- **Non-matching cards** (no aspect of that kind): dimmed hard — target ~0.25
  opacity with reduced contrast (vs. today's subtle 0.35 strip-level dim), the
  whole card, and they never auto-expand. They remain on the map (absence is
  information — "this object is missing this representation") but visibly out of
  the current view.
- Clearing the filter (`all`) collapses everything back to default and restores
  full contrast.

### 3. Filter at the chip tier

- Chips reflect the filter: non-matching chips dim exactly like non-matching
  cards; matching chips keep full contrast.
- On matching chips, the spine segment for the selected kind renders slightly
  wider/stronger so the match itself is visible at orbit.
- Hollow (zero-aspect) chips are by definition non-matching under any filter and
  take the hard dim.

### 4. Expansion-aware height estimation

- `estimateCardHeight` gains the current expansion context (effectively the
  active `kindFilter`): collapsed strips contribute header height only; an
  expanded strip contributes header + its field rows.
- Default (no filter): dramatically shorter cards → the masonry packs a far
  tighter, denser map (also resolving the known "airy gaps at headers tier"
  follow-up).
- With a filter active: matching strips' expanded heights are reserved, so the
  masonry re-packs for the filtered view. Unpinned cards reflow automatically
  (existing pinned/auto computed model — no architectural change); pinned cards
  stay put, accepting possible overlap as the cost of a manual pin.
- Manual click-expansion does NOT re-pack (positions must not jump under a
  click); the estimate tracks filter state only.

## Interaction summary

| State | Card with kind K | Card without kind K | Chip with K | Chip without K |
|---|---|---|---|---|
| No filter | collapsed strips | collapsed strips | normal | normal (hollow if empty) |
| Filter = K | K strip expanded, others collapsed | collapsed, hard-dimmed, never expands | full contrast, K spine emphasized | hard-dimmed |

## Testing

- EntityCard (Vitest): strips collapsed by default at full tier (field rows
  absent until toggled); filter expands exactly the matching strip; manual
  toggle overrides filter both directions; non-matching card carries the
  hard-dim class; chip dim + spine emphasis under filter.
- Autolayout: `estimateCardHeight` with no filter excludes field rows; with a
  filter includes them for matching aspects only; masonry no-overlap invariant
  holds in both states; determinism unchanged.
- Visual pass against the deep p3-edge store (~120 objects): default map is
  dense and readable; selecting each kind gives an unmistakable filtered view at
  chip and card tiers.

## Technical constraints

Unchanged: tokens-only styling (`--revision` reserved for selection; dimming via
opacity/desaturation, not new colors), Vue 3 + Vue Flow 1.48, pinned/auto layout
model, schema read-only.
