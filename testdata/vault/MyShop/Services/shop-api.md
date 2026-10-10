---
id: svc-k3x9qa
type: carto-service
description: The API server of the shop. It holds carts and orders, and publishes paid orders.
tags: []
created: 2026-09-29T09:00:00
updated: 2026-10-08T15:50:00
systems: ["[[Storefront]]"]
repository: github.com/myshop/myshop-api
path: server/
entry: ["server/cmd/api/main.go"]
runtime: Go 1.23
deploy: Cloud Run service shop-api
interfaces: ["[[Shop REST API]]", "[[Order Events]]", "[[Order History]]", "[[Stock Levels]]", "[[Stripe Payments]]", "[[Recs API]]"]
uses: ["[[shop-auth]]"]
stores: ["Cloud SQL shop-main"]
adopts: ["[[Shop Data Schema]]", "[[API Standards]]"]
compliance: 12/13
violated: 0
checked: 2026-10-08
---

## Purpose

Serves the REST API that shop-web calls. Stores carts and orders in its own database. Takes payments through Stripe. Publishes each paid order to Order Events, and exports orders to BigQuery each night.

## Requirements

## Obligations

```cartographer-obligations
```

> [!example] What the plugin shows
> | Clause | Rule | From | Status | Result |
> |---|---|---|---|---|
> | [[API Standards#API-1]] | The server handles 95 % of requests in … | adopts | unverified | inconclusive in [[2026-10-02 1500 Sweep#shop-api]] |
> | [[API Standards#API-2]] | The API accepts OAuth 2.0 bearer tokens and … | adopts | met · stale 1 commits, 7 days | [[2026-10-02 1500 Sweep#shop-api]] |
> | [[API Standards#API-3]] | An error response is a problem document as … | adopts | met | [[2026-10-02 1500 Sweep#shop-api]] |
> | [[Branch and CI Rules#CI-1]] | All work happens in a branch. Nobody commits … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#shop-api]] |
> | [[Branch and CI Rules#CI-2]] | A merge to preview runs the full test … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#shop-api]] |
> | [[Branch and CI Rules#CI-3]] | A branch merges to main only after its … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#shop-api]] |
> | [[Security Policy#SEC-1]] | Code reads its secrets from Secret Manager at … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#shop-api]] |
> | [[Security Policy#SEC-2]] | Every store of customer data uses the project's … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#shop-api]] |
> | [[Security Policy#SEC-3]] | Every endpoint that the internet can reach and … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#shop-api]] |
> | [[Security Policy#SEC-4]] | No log line holds a customer's name, email … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#shop-api]] |
> | [[Shop Data Schema#DS-1]] | Each payload that holds shop data validates against … | adopts | met · stale 1 commits, 7 days | [[2026-10-02 1500 Sweep#shop-api]] |
> | [[Shop Data Schema#DS-2]] | A price or a cost is an integer … | adopts | met · stale 1 commits, 7 days | [[2026-10-02 1500 Sweep#shop-api]] |
> | [[Shop Data Schema#DS-5]] | Timestamps are RFC 3339 strings in UTC. | adopts | met · stale 1 commits, 7 days | [[2026-10-02 1500 Sweep#shop-api]] |
>
> Compliance: met 12 / 13.
>
> Pending, for goals:
>
> | Clause | Goal | Result |
> |---|---|---|
> | [[Shop Data Schema#DS-4]] | [[Add currency to order lines]] | met on `goal/add-currency` |
> | [[Add currency to order lines#G-1]] | [[Add currency to order lines]] | unverified |

## Waivers

## Work

```cartographer-work
```

> [!example] What the plugin shows
> - T7 of [[Add currency to order lines]] (active): Test that an old order line reads back as USD. An agent works on it in myshop-api: [[2026-10-09 1012 e6f7a8]].
> - T1 of [[Make checkout survive a recommendation outage]] (draft): Call recs-server with a 300 ms timeout, and show no recommendations on a timeout. Open.
> - T2 of [[Make checkout survive a recommendation outage]] (draft): Test checkout with recs-server stopped. Open.
> - Question for you: [[How do we prove API-1]].

## Notes
