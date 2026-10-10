---
id: sys-r3c5ys
type: carto-system
description: Trains the product recommendation model each week and serves its results.
tags: []
created: 2026-09-29T09:00:00
updated: 2026-09-30T16:40:00
code: RECS
project: "[[MyShop]]"
adopts: ["[[Model Release Policy]]"]
compliance: 0/2
rollup: 24/26
violated: 0
---

## Purpose

Learns from order history which products customers buy together, and answers "what else would this customer like" for the shop.

## Requirements

### RECS-1
recs-server answers 99 % of requests in 50 ms or less.
[verify:: metric] [evidence:: Cloud Monitoring dashboard "recs latency"] [scope:: whole]

## Services

```cartographer-children
```

> [!example] What the plugin shows
> | Service | Compliance | Violated |
> |---|---|---|
> | [[recs-server]] | met 12 / 12 | 0 |
> | [[recs-trainer]] | met 12 / 12 | 0 |

## Obligations

```cartographer-obligations
```

> [!example] What the plugin shows
> | Clause | Rule | From | Status | Result |
> |---|---|---|---|---|
> | [[Model Release Policy#MRP-3]] | The previous model is live again within 5 … | adopts | unverified | not yet checked |
> | [[Recommendations#RECS-1]] | recs-server answers 99 % of requests in 50 … | local | unverified | not yet checked |
>
> Compliance: met 0 / 2. Not yet checked: in the sweep queue.

## Waivers



## Notes
