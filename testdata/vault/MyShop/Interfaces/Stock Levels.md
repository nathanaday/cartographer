---
id: ifc-st0ck5
type: carto-interface
description: A message for each change of a product's stock, on the Pub/Sub topic inventory.stock.
tags: []
created: 2026-09-29T09:00:00
updated: 2026-09-29T09:00:00
kind: event
definition: github.com/myshop/myshop-schemas/a12/stock.json
stability: stable
schemas: ["[[Shop Data Schema]]"]
external: ""
---

## Purpose

Lets the shop show whether a product is in stock.

## Wire

| | |
|---|---|
| Transport | Google Cloud Pub/Sub |
| Address | topic `inventory.stock` in project `myshop-prod` |
| Messages | `StockChanged`: the product id, the new level, and the time (`a12/stock.json`). |
| Delivery | At least once. The latest time wins. |
| Ordering | Ordered per product, with ordering key `product_id`. |
| Versioning | Fields are only added. |

`github.com/myshop/myshop-schemas/a12/stock.json` defines it in full. This document does not copy it.

## Attached services

```cartographer-attached
```

> [!example] What the plugin shows
> Attached: [[inventory-service]] · [[shop-api]]

## Notes
