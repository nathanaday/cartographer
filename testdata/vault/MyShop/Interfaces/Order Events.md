---
id: ifc-0rd3vt
type: carto-interface
description: A message for each paid order, on the Pub/Sub topic orders.paid.
tags: []
created: 2026-09-29T09:00:00
updated: 2026-09-29T09:00:00
kind: event
definition: github.com/myshop/myshop-schemas/a12/order.json
stability: stable
schemas: ["[[Shop Data Schema]]"]
external: ""
---

## Purpose

Tells the back office that a customer paid for an order.

## Wire

| | |
|---|---|
| Transport | Google Cloud Pub/Sub |
| Address | topic `orders.paid` in project `myshop-prod` |
| Messages | `OrderPaid`: one for each paid order. It holds the whole order (`a12/order.json`), with the attribute `order_id`. |
| Delivery | At least once. A reader ignores an `order_id` it has seen. |
| Ordering | None. |
| Versioning | Fields are only added. A breaking change gets a new topic, `orders.paid.v2`. |

`github.com/myshop/myshop-schemas/a12/order.json` defines it in full. This document does not copy it.

## Attached services

```cartographer-attached
```

> [!example] What the plugin shows
> Attached: [[inventory-service]] · [[invoice-worker]] · [[shop-api]]
>
> Lint: [[invoice-worker]] is attached to [[Order Events]], whose schema is [[Shop Data Schema]], but does not adopt it.

## Notes
