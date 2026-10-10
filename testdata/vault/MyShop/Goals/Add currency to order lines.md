---
id: goal-c5v2lw
type: carto-goal
description: Every order line states its currency, so the shop can sell in euros next quarter.
tags: []
created: 2026-10-05T16:20:00
updated: 2026-10-09T10:40:00
status: active
priority: high
after: []
defer: ""
branch: goal/add-currency
resolves: []
affects: ["[[inventory-service]]", "[[recs-trainer]]", "[[shop-api]]", "[[shop-web]]"]
progress: 2/5
tasks: 4/8
landed: 1/4
opened: 2026-10-06
closed: ""
---

## Intent

> I want to start selling in euros next quarter. Orders need to say what currency they are in. Don't do conversion or display yet, just get the data right everywhere.

## Map changes

- **add** [[Shop Data Schema#DS-4]] · Each order line carries a `currency` field with an ISO 4217 code. A reader that finds no `currency` treats the line as `USD`.

## Requirements

### G-1
An order line stored before the change reads back with currency `USD`.
[verify:: test] [on:: [[shop-api]]] [rev:: 1]

## Decisions

- The currency is stored per order line, so that a cart with more than one currency works later. From [[Store currency per line or per order]].
- A reader that finds no `currency` uses `USD`. This is part of DS-4, so old messages on Order Events stay valid.

## Out of scope

- Conversion between currencies.
- Showing a currency other than USD in shop-web.

## Tasks

- [x] T1 [[Shop Data Schema]] Add `currency` to `a12/order.json`, with the USD default [for:: DS-4] [commit:: myshop-schemas@e41c0a2] [done:: 2026-10-07] ^t1
- [x] T2 [[shop-api]] Add `currency` to the order line model, with a migration that sets USD on old lines [for:: DS-4, G-1] [commit:: 5dc85ea] [done:: 2026-10-08] ^t2
- [x] T3 [[shop-api]] Send `currency` in the REST API and on Order Events, and update `openapi.yaml` [for:: DS-4] [commit:: 08fd415] [done:: 2026-10-08] ^t3
- [ ] T4 [[inventory-service]] Read `currency` from Order Events and keep it on the reservation [for:: DS-4] [after:: T3] ^t4
- [ ] T5 [[shop-web]] Parse `currency` on the order pages and keep the USD display [for:: DS-4] [after:: T3] ^t5
- [ ] T6 [[invoice-worker]] Print the currency on each invoice line [for:: DS-4] [blocked:: [[Should invoice-worker adopt the Shop Data Schema]]] ^t6
- [ ] T7 [[shop-api]] Test that an old order line reads back as USD [for:: G-1] ^t7
- [x] T8 [[shop-api]] Add `currency` to the nightly Order History export [for:: DS-4] [commit:: 08fd415] [done:: 2026-10-08] ^t8

## Task details

### T4
`inventory/consumer.py` decodes Order Events into a dataclass and drops fields it does not know. Add the field. The reservation table needs a column, so add a migration.

### T8
Found by the review of the draft: `export/orders_table.sql` names its columns, so the new field would not reach Order History, and recs-trainer would never see it.

## Progress

```cartographer-progress
```

> [!example] What the plugin shows
> **obligations 2/5 · tasks 4/8 · landed 1/4 · waiting on you: 1 question**
>
> | Clause | On | Result | Last check |
> |---|---|---|---|
> | [[Add currency to order lines#G-1]] | [[shop-api]] | inconclusive on `goal/add-currency` | [[2026-10-08 1550 shop-api · 08fd415]] |
> | [[Shop Data Schema#DS-4]] | [[inventory-service]] | unverified | none |
> | [[Shop Data Schema#DS-4]] | [[recs-trainer]] | met | [[2026-10-08 1630 recs-trainer · 5c6d7e8]] |
> | [[Shop Data Schema#DS-4]] | [[shop-api]] | met on `goal/add-currency` | [[2026-10-08 1550 shop-api · 08fd415]] |
> | [[Shop Data Schema#DS-4]] | [[shop-web]] | unverified | none |
>
> Branch `goal/add-currency`: myshop-api open · myshop-inventory open · myshop-schemas landed · myshop-web open
>
> Waiting on you: [[Should invoice-worker adopt the Shop Data Schema]]
>
> Agents: T4 in myshop-inventory: [[2026-10-09 1012 c4d5e6]] · T5 in myshop-web: [[2026-10-09 1012 d5e6f7]] · T7 in myshop-api: [[2026-10-09 1012 e6f7a8]]

## Log

- 2026-10-05 16:20 drafted by the agent from the intent. The map gives 4 affected services: every adopter of [[Shop Data Schema]].
- 2026-10-05 16:25 question for the user: [[Store currency per line or per order]].
- 2026-10-05 16:30 lint on the draft: L1 on [[invoice-worker]]. Question for the user: [[Should invoice-worker adopt the Shop Data Schema]]. T6 waits on it.
- 2026-10-06 09:10 the user answered the first question. DS-4 gets the USD default. G-1 added.
- 2026-10-06 09:40 review of the draft by a fresh agent: 1 finding, the Order History export. T8 added.
- 2026-10-06 14:05 the user approved the goal. DS-4 is pending in [[Shop Data Schema]]. The invoice-worker question stays open, and T6 waits.
- 2026-10-07 11:30 T1 done in myshop-schemas. The change is compatible (a new optional field), so it lands first.
- 2026-10-07 11:38 the reviewer passed the definition change. Pull request myshop-schemas#31 opened.
- 2026-10-07 11:40 the user approved the Land line. landed myshop-schemas e41c0a2
- 2026-10-08 15:50 T2, T3, T8 done on the goal branch in myshop-api. Check [[2026-10-08 1550 shop-api · 08fd415]]: DS-4 met. G-1 inconclusive until T7.
- 2026-10-08 16:30 check [[2026-10-08 1630 recs-trainer · 5c6d7e8]]: DS-4 met with no task.
- 2026-10-09 10:12 the runner started T4 in myshop-inventory, T5 in myshop-web, and T7 in myshop-api, each in its own worktree: [[2026-10-09 1012 c4d5e6]], [[2026-10-09 1012 d5e6f7]], [[2026-10-09 1012 e6f7a8]].
