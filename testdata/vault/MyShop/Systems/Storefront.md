---
id: sys-p2q8rt
type: carto-system
description: The customer-facing shop, from the product page to the paid order.
tags: []
created: 2026-09-29T09:00:00
updated: 2026-10-02T15:10:00
code: SHOP
project: "[[MyShop]]"
adopts: []
compliance: 1/2
rollup: 35/37
violated: 0
---

## Purpose

A customer finds a product, sees recommendations, adds products to the cart, and pays. The system ends when the paid order is stored and published.

## Requirements

### SHOP-1
A paid order reaches the inventory system within 5 s.
[verify:: test] [evidence:: myshop-web e2e/checkout.spec.ts] [scope:: whole]

### SHOP-2
A customer can pay when recs-server is down.
[verify:: test] [evidence:: myshop-web e2e/checkout-no-recs.spec.ts] [scope:: whole]

## Services

```cartographer-children
```

> [!example] What the plugin shows
> | Service | Compliance | Violated |
> |---|---|---|
> | [[recs-server]] | met 12 / 12 | 0 |
> | [[shop-api]] | met 12 / 13 | 0 |
> | [[shop-web]] | met 10 / 10 | 0 |

## Obligations

```cartographer-obligations
```

> [!example] What the plugin shows
> | Clause | Rule | From | Status | Result |
> |---|---|---|---|---|
> | [[Storefront#SHOP-1]] | A paid order reaches the inventory system within … | local | met · stale 5 commits, 6 days | [[2026-10-02 1500 Sweep#Storefront]] |
> | [[Storefront#SHOP-2]] | A customer can pay when recs-server is down. | local | waived (violated) · stale 5 commits, 6 days | [[2026-10-02 1500 Sweep#Storefront]] |
>
> Compliance: met 1 · waived 1 / 2.

## Waivers

- [[Storefront#SHOP-2]] · shop-api waits for recs-server during checkout with no timeout. [until:: 2026-11-15] [goal:: [[Make checkout survive a recommendation outage]]] ^w1

## Notes
