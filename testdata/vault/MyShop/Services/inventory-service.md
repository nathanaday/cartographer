---
id: svc-1nv3nt
type: carto-service
description: Keeps the stock of every product and reserves stock for each paid order.
tags: []
created: 2026-09-29T09:00:00
updated: 2026-09-29T09:00:00
systems: ["[[Back Office]]"]
repository: github.com/myshop/myshop-inventory
path: inventory/
entry: ["inventory/main.py"]
runtime: Python 3.12, FastAPI
deploy: Cloud Run service inventory
interfaces: ["[[Stock Levels]]", "[[Order Events]]"]
uses: ["[[myshop-common]]"]
stores: ["Cloud SQL inventory"]
adopts: ["[[Shop Data Schema]]"]
compliance: 9/10
violated: 1
checked: 2026-10-02
---

## Purpose

Reads each paid order from Order Events and reserves its stock. Publishes the new stock level of each product that changed. Has an admin API for the warehouse team.

## Requirements

## Obligations

```cartographer-obligations
```

> [!example] What the plugin shows
> | Clause | Rule | From | Status | Result |
> |---|---|---|---|---|
> | [[Branch and CI Rules#CI-1]] | All work happens in a branch. Nobody commits … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#inventory-service]] |
> | [[Branch and CI Rules#CI-2]] | A merge to preview runs the full test … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#inventory-service]] |
> | [[Branch and CI Rules#CI-3]] | A branch merges to main only after its … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#inventory-service]] |
> | [[Security Policy#SEC-1]] | Code reads its secrets from Secret Manager at … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#inventory-service]] |
> | [[Security Policy#SEC-2]] | Every store of customer data uses the project's … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#inventory-service]] |
> | [[Security Policy#SEC-3]] | Every endpoint that the internet can reach and … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#inventory-service]] |
> | [[Security Policy#SEC-4]] | No log line holds a customer's name, email … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#inventory-service]] |
> | [[Shop Data Schema#DS-1]] | Each payload that holds shop data validates against … | adopts | met · stale 1 commits, 6 days | [[2026-10-02 1500 Sweep#inventory-service]] |
> | [[Shop Data Schema#DS-2]] | A price or a cost is an integer … | adopts | **violated** · stale 1 commits, 6 days | [[2026-10-02 1500 Sweep#inventory-service]] |
> | [[Shop Data Schema#DS-5]] | Timestamps are RFC 3339 strings in UTC. | adopts | met · stale 1 commits, 6 days | [[2026-10-02 1500 Sweep#inventory-service]] |
>
> Compliance: met 9 / 10.
>
> Pending, for goals:
>
> | Clause | Goal | Result |
> |---|---|---|
> | [[Shop Data Schema#DS-4]] | [[Add currency to order lines]] | unverified |
>
> Open findings: [[DS-2 violated in inventory-service]]

## Waivers

## Work

```cartographer-work
```

> [!example] What the plugin shows
> - T4 of [[Add currency to order lines]] (active): Read `currency` from Order Events and keep it on the reservation. An agent works on it in myshop-inventory: [[2026-10-09 1012 c4d5e6]].
> - Open finding: [[DS-2 violated in inventory-service]].

## Notes
