---
id: prj-m7shop
type: carto-project
description: The online shop, its back office, and the recommendation pipeline.
tags: []
created: 2026-09-29T09:00:00
updated: 2026-10-09T10:40:00
adopts: ["[[Security Policy]]", "[[Branch and CI Rules]]"]
repositories: ["github.com/myshop/myshop-web", "github.com/myshop/myshop-api", "github.com/myshop/myshop-inventory", "github.com/myshop/myshop-recs", "github.com/myshop/myshop-schemas", "github.com/myshop/myshop-common", "github.com/myshop/myshop-infra"]
platform: ["Secret Manager", "Cloud Logging", "Cloud Monitoring", "Identity Platform", "Identity-Aware Proxy"]
default_branch: main
landing: user
ci_jobs: []
max_agents: 3
task_budget: 120
stale_after: 30
nudge_after: 14
canvas: "[[MyShop.canvas]]"
rollup: 70/83
violated: 1
---

## Purpose

Customers browse products, get recommendations, and pay. The back office keeps stock and writes invoices. The recommendation pipeline trains a model each week from order history.

## Since your last visit

```cartographer-since
```

> [!example] What the plugin shows
> Since 2026-10-06 18:00. **Mark seen**
>
> - [[Add currency to order lines]]: obligations 0/5 → 2/5, tasks 0/8 → 4/8.
> - Landed: myshop-schemas of [[Add currency to order lines]], at e41c0a2.
> - Newly met on the default branch: [[recs-trainer]] (12, its first check). Newly violated: none.
> - Findings opened: [[reindex job is in no entity]]. Resolved: none.
> - Questions answered: none.

## Attention

```cartographer-attention
```

> [!example] What the plugin shows
> - **Land** T2 of [[Upkeep]]: Store unit cost as integer cents at import, and convert the existing rows. Done at 3c4d5e6; myshop-inventory, branch `upkeep/t2`.
>   Land · Request changes · **Not now until…**
> - **Answer** [[Should invoice-worker adopt the Shop Data Schema]]. Unblocks T6 of [[Add currency to order lines]].
>   1. Adopt ★ · 2. Do not adopt, and do T6 anyway · 3. Do not adopt, and drop T6 · **Accept recommended** · **Not now until…**
> - **Decide** on [[reindex job is in no entity]].
>   1. Add a service reindex-job to Storefront ★ · 2. Add it to the `entry` of shop-api · 3. Waive · **Accept recommended** · **Not now until…**
> - **Answer** [[How do we prove API-1]]. Blocks nothing.
>   1. A metric, as recs-server does ★ · 2. A load test in preview CI · 3. Waive API-1 on shop-api · **Accept recommended** · **Not now until…**
>
> Deferred: 1 (until 2026-11-01).

## Goals

```cartographer-goals
```

> [!example] What the plugin shows
> | Goal | Status | Progress | Ready | Waiting on you | Agents | Last activity |
> |---|---|---|---|---|---|---|
> | [[Add currency to order lines]] | active | obligations 2/5 · tasks 4/8 · landed 1/4 | — | [[Should invoice-worker adopt the Shop Data Schema]] | T4 in myshop-inventory: [[2026-10-09 1012 c4d5e6]] · T5 in myshop-web: [[2026-10-09 1012 d5e6f7]] · T7 in myshop-api: [[2026-10-09 1012 e6f7a8]] | 2026-10-09 10:40 |
> | [[Make checkout survive a recommendation outage]] | draft, deferred until 2026-11-01 | obligations 0/2 · tasks 0/3 | — | approval | none | 2026-10-02 15:12 |
> | [[Upkeep]] | standing | tasks 2/3 · open 1 | T3 (no free slot) | — | none | 2026-10-03 09:20 |
>
> Run queue: 1 ready task · 3 agents at work of 3 · 22 obligations to sweep, next sweep 2026-10-10 02:00. **Run now**

## Health

```cartographer-health
```

> [!example] What the plugin shows
> - Violation: [[DS-2 violated in inventory-service]] → Upkeep T2, done, waiting for you to land.
> - Violation: [[SHOP-2 violated in Storefront]] · waived until 2026-11-15 → [[Make checkout survive a recommendation outage]] (draft, deferred until 2026-11-01).
> - Unmapped: [[reindex job is in no entity]].
> - Inconclusive: [[API Standards#API-1]] on [[shop-api]] → [[How do we prove API-1]].
> - Lint L1: [[invoice-worker]] is attached to [[Order Events]], whose schema is [[Shop Data Schema]], but does not adopt it.
> - Never checked: [[Back Office]] · [[Recommendations]] · [[invoice-worker]].
> - Sweep queue: 22 obligations (12 stale, oldest 7 days), next sweep 2026-10-10 02:00.

## Waivers
