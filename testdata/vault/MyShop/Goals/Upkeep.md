---
id: goal-upk33p
type: carto-goal
description: Work in MyShop that belongs to no other goal.
tags: []
created: 2026-09-29T09:00:00
updated: 2026-10-03T09:20:00
status: active
standing: True
priority: medium
after: []
defer: ""
branch: ""
resolves: []
affects: []
progress: 0/0
tasks: 2/3
landed: ""
opened: 2026-09-29
closed: ""
---

## Intent

The project's standing goal. Each task is already the user's decision, so it needs no approval and never closes. Each task has its own branch, `upkeep/t<n>`, and lands alone.

## Tasks

- [x] T1 [[shop-api]] Remove the `tax_included` field after DS-3 was retired [by:: user] [commit:: 2a7c9e4] [done:: 2026-09-30] ^t1
- [x] T2 [[inventory-service]] Store unit cost as integer cents at import, and convert the existing rows [finding:: [[DS-2 violated in inventory-service]]] [commit:: 3c4d5e6] [done:: 2026-10-03] ^t2
- [ ] T3 [[shop-web]] Upgrade Node 18 to 22 before its end of support [by:: user] [due:: 2026-12-01] ^t3

## Task details

### T2
`stock/valuation.py:58` multiplies `unit_cost` (a float from the warehouse CSV import) by quantity. Store cents as an integer at import, and convert the 1,840 existing rows.

## Progress

```cartographer-progress
```

> [!example] What the plugin shows
> **tasks 2/3 · open 1**
>
>
> Ready for the run queue: T3

## Log

- 2026-09-30 15:00 T1 done by the user's request after DS-3 was retired. landed T1 myshop-api 2a7c9e4
- 2026-10-02 11:52 T2 added from [[DS-2 violated in inventory-service]], option 1. The runner started it at once: [[2026-10-02 1153 b3c4d5]].
- 2026-10-03 09:20 T2 done on `upkeep/t2` in myshop-inventory. Check on the branch: DS-2 met. The reviewer passed. Pull request myshop-inventory#57 opened. It waits for the user to land it.
