---
id: fnd-d52inv
type: carto-finding
description: inventory-service stores unit cost as a float, and DS-2 requires integer cents.
tags: []
created: 2026-10-02T11:40:00
updated: 2026-10-02T11:52:00
kind: violation
entity: "[[inventory-service]]"
clause: "[[Shop Data Schema#DS-2]]"
about: ""
key: ""
check: "[[2026-10-02 1500 Sweep#inventory-service]]"
found: 2026-10-02
defer: ""
status: open
contract: "[[Shop Data Schema]]"
resolution: ["[[Upkeep#^t2]]"]
resolved: ""
---

## Finding

[[Shop Data Schema#DS-2]] requires every price and cost to be an integer number of cents. [[inventory-service]] stores `unit_cost` as a float.

## Evidence

- `stock/valuation.py:58` multiplies `unit_cost` by the quantity. `unit_cost` comes as a float from the warehouse CSV import, `stock/importer.py:31`.
- 1,840 rows in table `stock_items` hold a float cost.

## Options

1. **Add an upkeep task** (recommended). The runner starts it at once.
   Change: add a task for [[inventory-service]] to [[Upkeep]], with this finding as its source.
2. **Start a goal.**
3. **Waive until a date.**
   Change: add a waiver of [[Shop Data Schema#DS-2]] to [[inventory-service]].

[chosen:: 1] [by:: user] [on:: 2026-10-02]

## History

- 2026-10-02 11:40 opened by [[2026-10-02 1500 Sweep#inventory-service]].
- 2026-10-02 11:52 the user chose option 1: task T2 of [[Upkeep]].
