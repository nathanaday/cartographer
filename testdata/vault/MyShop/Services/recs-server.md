---
id: svc-r3csrv
type: carto-service
description: Serves product recommendations from the live model.
tags: []
created: 2026-09-29T09:00:00
updated: 2026-09-29T09:00:00
systems: ["[[Storefront]]", "[[Recommendations]]"]
repository: github.com/myshop/myshop-recs
path: server/
entry: ["server/app.py"]
runtime: Python 3.12, FastAPI
deploy: Cloud Run service recs-server
interfaces: ["[[Recs API]]", "[[Recs Model]]"]
uses: ["[[myshop-common]]"]
stores: []
adopts: ["[[API Standards]]"]
compliance: 12/12
violated: 0
checked: 2026-10-02
---

## Purpose

Loads the model version that is marked live, and answers "what else would this customer like" for a customer and a cart.

## Requirements

## Obligations

```cartographer-obligations
```

> [!example] What the plugin shows
> | Clause | Rule | From | Status | Result |
> |---|---|---|---|---|
> | [[API Standards#API-1]] | The server handles 95 % of requests in … | adopts | met | [[2026-10-02 1500 Sweep#recs-server]] |
> | [[API Standards#API-2]] | The API accepts OAuth 2.0 bearer tokens and … | adopts | met | [[2026-10-02 1500 Sweep#recs-server]] |
> | [[API Standards#API-3]] | An error response is a problem document as … | adopts | met | [[2026-10-02 1500 Sweep#recs-server]] |
> | [[Branch and CI Rules#CI-1]] | All work happens in a branch. Nobody commits … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#recs-server]] |
> | [[Branch and CI Rules#CI-2]] | A merge to preview runs the full test … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#recs-server]] |
> | [[Branch and CI Rules#CI-3]] | A branch merges to main only after its … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#recs-server]] |
> | [[Model Release Policy#MRP-1]] | Each model version records its training data snapshot, … | [[Recommendations]] | met | [[2026-10-02 1500 Sweep#recs-server]] |
> | [[Model Release Policy#MRP-2]] | A new model becomes live only if its … | [[Recommendations]] | met | [[2026-10-02 1500 Sweep#recs-server]] |
> | [[Security Policy#SEC-1]] | Code reads its secrets from Secret Manager at … | [[MyShop]] | met (none) | [[2026-10-02 1500 Sweep#recs-server]] |
> | [[Security Policy#SEC-2]] | Every store of customer data uses the project's … | [[MyShop]] | met (none) | [[2026-10-02 1500 Sweep#recs-server]] |
> | [[Security Policy#SEC-3]] | Every endpoint that the internet can reach and … | [[MyShop]] | met (none) | [[2026-10-02 1500 Sweep#recs-server]] |
> | [[Security Policy#SEC-4]] | No log line holds a customer's name, email … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#recs-server]] |
>
> Compliance: met 12 / 12.

## Waivers

## Work

```cartographer-work
```

> [!example] What the plugin shows
> No open work.

## Notes

- Belongs to two systems: customers reach it through the Storefront, and the Recommendations system owns its model.
