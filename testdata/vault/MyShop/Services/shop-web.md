---
id: svc-w3bf0e
type: carto-service
description: The web front end of the shop, built with Next.js.
tags: []
created: 2026-09-29T09:00:00
updated: 2026-09-29T09:00:00
systems: ["[[Storefront]]"]
repository: github.com/myshop/myshop-web
path: ./
entry: ["app/layout.tsx (next start)"]
runtime: Node 18, Next.js 14
deploy: Cloud Run service shop-web
interfaces: ["[[Shop REST API]]", "[[Recs API]]"]
uses: []
stores: []
adopts: ["[[Shop Data Schema]]"]
compliance: 10/10
violated: 0
checked: 2026-10-08
---

## Purpose

Renders the product pages, the cart, and the checkout for customers. Its server routes call shop-api and recs-server.

## Requirements

## Obligations

```cartographer-obligations
```

> [!example] What the plugin shows
> | Clause | Rule | From | Status | Result |
> |---|---|---|---|---|
> | [[Branch and CI Rules#CI-1]] | All work happens in a branch. Nobody commits … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#shop-web]] |
> | [[Branch and CI Rules#CI-2]] | A merge to preview runs the full test … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#shop-web]] |
> | [[Branch and CI Rules#CI-3]] | A branch merges to main only after its … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#shop-web]] |
> | [[Security Policy#SEC-1]] | Code reads its secrets from Secret Manager at … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#shop-web]] |
> | [[Security Policy#SEC-2]] | Every store of customer data uses the project's … | [[MyShop]] | met (none) | [[2026-10-08 0200 Sweep#shop-web]] |
> | [[Security Policy#SEC-3]] | Every endpoint that the internet can reach and … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#shop-web]] |
> | [[Security Policy#SEC-4]] | No log line holds a customer's name, email … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#shop-web]] |
> | [[Shop Data Schema#DS-1]] | Each payload that holds shop data validates against … | adopts | met | [[2026-10-08 0200 Sweep#shop-web]] |
> | [[Shop Data Schema#DS-2]] | A price or a cost is an integer … | adopts | met | [[2026-10-08 0200 Sweep#shop-web]] |
> | [[Shop Data Schema#DS-5]] | Timestamps are RFC 3339 strings in UTC. | adopts | met | [[2026-10-08 0200 Sweep#shop-web]] |
>
> Compliance: met 10 / 10.
>
> Pending, for goals:
>
> | Clause | Goal | Result |
> |---|---|---|
> | [[Shop Data Schema#DS-4]] | [[Add currency to order lines]] | unverified |

## Waivers

## Work

```cartographer-work
```

> [!example] What the plugin shows
> - T5 of [[Add currency to order lines]] (active): Parse `currency` on the order pages and keep the USD display. An agent works on it in myshop-web: [[2026-10-09 1012 d5e6f7]].
> - T3 of [[Make checkout survive a recommendation outage]] (draft): Run `e2e/checkout-no-recs.spec.ts` in preview CI. Open.
> - T3 of [[Upkeep]] (active): Upgrade Node 18 to 22 before its end of support. Open.

## Notes
