---
id: svc-r3ctrn
type: carto-service
description: Trains the recommendation model each week from order history.
tags: []
created: 2026-09-29T09:00:00
updated: 2026-09-29T09:00:00
systems: ["[[Recommendations]]"]
repository: github.com/myshop/myshop-recs
path: trainer/
entry: ["trainer/train.py"]
runtime: Python 3.12, PyTorch Geometric
deploy: Vertex AI custom job, each Monday 03:00 UTC
interfaces: ["[[Recs Model]]", "[[Order History]]"]
uses: ["[[myshop-common]]"]
stores: []
adopts: ["[[Shop Data Schema]]"]
compliance: 12/12
violated: 0
checked: 2026-10-08
---

## Purpose

Builds a graph of customers and products from Order History, trains a graph neural network on it, and publishes a new model version when it beats the live one.

## Requirements

## Obligations

```cartographer-obligations
```

> [!example] What the plugin shows
> | Clause | Rule | From | Status | Result |
> |---|---|---|---|---|
> | [[Branch and CI Rules#CI-1]] | All work happens in a branch. Nobody commits … | [[MyShop]] | met | [[2026-10-08 1630 recs-trainer · 5c6d7e8]] |
> | [[Branch and CI Rules#CI-2]] | A merge to preview runs the full test … | [[MyShop]] | met | [[2026-10-08 1630 recs-trainer · 5c6d7e8]] |
> | [[Branch and CI Rules#CI-3]] | A branch merges to main only after its … | [[MyShop]] | met | [[2026-10-08 1630 recs-trainer · 5c6d7e8]] |
> | [[Model Release Policy#MRP-1]] | Each model version records its training data snapshot, … | [[Recommendations]] | met | [[2026-10-08 1630 recs-trainer · 5c6d7e8]] |
> | [[Model Release Policy#MRP-2]] | A new model becomes live only if its … | [[Recommendations]] | met | [[2026-10-08 1630 recs-trainer · 5c6d7e8]] |
> | [[Security Policy#SEC-1]] | Code reads its secrets from Secret Manager at … | [[MyShop]] | met (none) | [[2026-10-08 1630 recs-trainer · 5c6d7e8]] |
> | [[Security Policy#SEC-2]] | Every store of customer data uses the project's … | [[MyShop]] | met | [[2026-10-08 1630 recs-trainer · 5c6d7e8]] |
> | [[Security Policy#SEC-3]] | Every endpoint that the internet can reach and … | [[MyShop]] | met (none) | [[2026-10-08 1630 recs-trainer · 5c6d7e8]] |
> | [[Security Policy#SEC-4]] | No log line holds a customer's name, email … | [[MyShop]] | met | [[2026-10-08 1630 recs-trainer · 5c6d7e8]] |
> | [[Shop Data Schema#DS-1]] | Each payload that holds shop data validates against … | adopts | met | [[2026-10-08 1630 recs-trainer · 5c6d7e8]] |
> | [[Shop Data Schema#DS-2]] | A price or a cost is an integer … | adopts | met | [[2026-10-08 1630 recs-trainer · 5c6d7e8]] |
> | [[Shop Data Schema#DS-5]] | Timestamps are RFC 3339 strings in UTC. | adopts | met | [[2026-10-08 1630 recs-trainer · 5c6d7e8]] |
>
> Compliance: met 12 / 12.
>
> Pending, for goals:
>
> | Clause | Goal | Result |
> |---|---|---|
> | [[Shop Data Schema#DS-4]] | [[Add currency to order lines]] | met |

## Waivers

## Work

```cartographer-work
```

> [!example] What the plugin shows
> No open work.

## Notes
