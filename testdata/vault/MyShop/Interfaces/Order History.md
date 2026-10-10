---
id: ifc-0rdh1s
type: carto-interface
description: All orders, exported each night to the BigQuery table analytics.orders.
tags: []
created: 2026-09-29T09:00:00
updated: 2026-09-29T09:00:00
kind: dataset
definition: github.com/myshop/myshop-api/server/export/orders_table.sql
stability: stable
schemas: ["[[Shop Data Schema]]"]
external: ""
---

## Purpose

Gives analysis and training jobs every order, with no load on the shop database.

## Wire

| | |
|---|---|
| Transport | BigQuery |
| Address | table `myshop-prod.analytics.orders`, partitioned by the day of `paid_at` |
| Messages | One row for each order line. The columns are in `export/orders_table.sql`. |
| Delivery | A scheduled query replaces the previous day's partition each night at 02:00 UTC. |
| Versioning | Columns are only added. |

`github.com/myshop/myshop-api/server/export/orders_table.sql` defines it in full. This document does not copy it.

## Attached services

```cartographer-attached
```

> [!example] What the plugin shows
> Attached: [[recs-trainer]] · [[shop-api]]

## Notes
