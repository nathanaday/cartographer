---
id: fnd-apirec
type: carto-finding
description: The code of shop-api calls Recs API, and the map does not attach shop-api to it.
tags: []
created: 2026-10-01T17:30:00
updated: 2026-10-02T15:10:00
kind: divergence
entity: "[[shop-api]]"
clause: ""
about: "[[Recs API]]"
key: https://recs.myshop.internal
check: "[[2026-10-02 1500 Sweep#shop-api]]"
found: 2026-10-01
defer: ""
status: resolved
contract: ""
resolution: []
resolved: 2026-10-02
---

## Finding

[[shop-api]] calls [[Recs API]], but its `interfaces` field does not list Recs API. The map hides a dependency: a change to Recs API would not show shop-api as affected.

## Evidence

- `server/checkout/recs.go:31` calls `https://recs.myshop.internal/v1/recs` during checkout.

## Options

1. **Attach it on the map now** (recommended). The map is then true at once, and a change to Recs API shows shop-api as affected.
   Change: add [[Recs API]] to `interfaces` of [[shop-api]].
2. **Remove the call** from the code, as an upkeep task.
3. **Waive until a date.**

[chosen:: 1] [by:: user] [on:: 2026-10-02]

The finding resolved when the map declared the dependency. What the call does during checkout is a separate problem: [[SHOP-2 violated in Storefront]].

## History

- 2026-10-01 17:30 opened by [[2026-10-02 1500 Sweep#shop-api]].
- 2026-10-02 15:10 the user chose option 1. shop-api is attached to Recs API, and the finding resolved.
