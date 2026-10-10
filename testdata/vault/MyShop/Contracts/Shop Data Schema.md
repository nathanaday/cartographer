---
id: con-x2b6mn
type: carto-contract
description: The shape of product, order, and stock data in every service that sends or reads it.
tags: []
created: 2026-09-29T09:00:00
updated: 2026-10-06T14:05:00
code: DS
kind: schema
definition: github.com/myshop/myshop-schemas/a12/
status: active
---

## Purpose

Keeps every service that sends or reads shop data on one shape of that data. The JSON Schema files in myshop-schemas define the shape. This contract states the rules about it.

## Clauses

### DS-1
Each payload that holds shop data validates against the matching file in `myshop-schemas/a12/`.
[verify:: test] [evidence:: a schema conformance test in each repository] [rev:: 2]

### DS-2
A price or a cost is an integer number of cents, never a float.


### DS-3
~~Prices carry a `tax_included` flag.~~
[state:: retired] [retired:: 2026-09-30]

### DS-4
Each order line carries a `currency` field with an ISO 4217 code. A reader that finds no `currency` treats the line as `USD`.
[verify:: test] [state:: pending] [goal:: [[Add currency to order lines]]]

### DS-5
Timestamps are RFC 3339 strings in UTC.


## Adopters

```cartographer-adopters
```

> [!example] What the plugin shows
> | Entity | DS-1 | DS-2 | DS-4 (pending) | DS-5 |
> |---|---|---|---|---|
> | [[inventory-service]] | met · stale | **violated** · stale | unverified | met · stale |
> | [[recs-trainer]] | met | met | met | met |
> | [[shop-api]] | met · stale | met · stale | met (branch) | met · stale |
> | [[shop-web]] | met | met | unverified | met |
>
> Adopted by [[inventory-service]] · [[recs-trainer]] · [[shop-api]] · [[shop-web]]. **Adopt for…**
>
> Open findings: [[DS-2 violated in inventory-service]]
>
> Lint: [[invoice-worker]] is attached to [[Order Events]], whose schema is [[Shop Data Schema]], but does not adopt it.

## History

- 2026-09-30: DS-1 revised to rev 2. The schema files moved from `myshop-api/schema/` to `myshop-schemas/a12/`.
- 2026-09-30: DS-3 retired. Prices no longer carry a tax flag.
- 2026-10-06: DS-4 added as pending by [[Add currency to order lines]].
