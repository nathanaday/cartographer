---
id: ifc-r3cap1
type: carto-interface
description: The HTTP API of recs-server, for recommendations for a customer and a cart.
tags: []
created: 2026-09-29T09:00:00
updated: 2026-09-29T09:00:00
kind: http
definition: github.com/myshop/myshop-recs/server/openapi.yaml
stability: stable
schemas: []
external: ""
---

## Purpose

Returns the products that a customer is likely to buy next.

## Wire

| | |
|---|---|
| Transport | HTTPS, JSON |
| Address | `https://recs.myshop.internal/v1`, internal only |
| Messages | `GET /v1/recs?customer_id=…&cart=…` returns up to 10 product ids with scores. |
| Auth | A service bearer token. |
| Versioning | The path holds the major version. |

`github.com/myshop/myshop-recs/server/openapi.yaml` defines it in full. This document does not copy it.

## Attached services

```cartographer-attached
```

> [!example] What the plugin shows
> Attached: [[recs-server]] · [[shop-api]] · [[shop-web]]

## Notes
