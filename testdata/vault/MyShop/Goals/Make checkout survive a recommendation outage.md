---
id: goal-r3c0ut
type: carto-goal
description: A customer can pay when recs-server is down.
tags: []
created: 2026-10-02T15:10:00
updated: 2026-10-02T15:12:00
status: draft
priority: medium
after: []
defer: 2026-11-01
branch: goal/checkout-without-recs
resolves: ["[[SHOP-2 violated in Storefront]]"]
affects: ["[[Storefront]]", "[[shop-api]]"]
progress: 0/2
tasks: 0/3
landed: 0/2
opened: ""
closed: ""
---

## Intent

From [[SHOP-2 violated in Storefront]], option 4:

> Waive it for now, but draft the fix so it's ready when I have time.

## Map changes

None. shop-api was attached to [[Recs API]] on 2026-10-02, from [[shop-api calls Recs API without attaching it]].

## Requirements

### G-1
The checkout endpoint answers within 2 s when recs-server does not answer.
[verify:: test] [on:: [[shop-api]]] [rev:: 1]

The goal resolves [[SHOP-2 violated in Storefront]], so [[Storefront#SHOP-2]] counts toward it. Its waiver ends when the goal closes.

## Decisions

## Out of scope

- A cache of recommendations in shop-api.

## Tasks

- [ ] T1 [[shop-api]] Call recs-server with a 300 ms timeout, and show no recommendations on a timeout [for:: G-1] ^t1
- [ ] T2 [[shop-api]] Test checkout with recs-server stopped [for:: G-1] ^t2
- [ ] T3 [[shop-web]] Run `e2e/checkout-no-recs.spec.ts` in preview CI [for:: SHOP-2] ^t3

## Progress

```cartographer-progress
```

> [!example] What the plugin shows
> **obligations 0/2 · tasks 0/3 · waiting on you: approval, deferred until 2026-11-01**
>
> | Clause | On | Result | Last check |
> |---|---|---|---|
> | [[Make checkout survive a recommendation outage#G-1]] | [[shop-api]] | unverified | none |
> | [[Storefront#SHOP-2]] | [[Storefront]] | waived (does not count) | [[2026-10-02 1500 Sweep#Storefront]] |
>
> Branch `goal/checkout-without-recs`: myshop-api open · myshop-web open

## Log

- 2026-10-02 15:10 drafted by the agent from [[SHOP-2 violated in Storefront]]. Lint on the draft: no results.
- 2026-10-02 15:12 the user chose "Not now until 2026-11-01".
