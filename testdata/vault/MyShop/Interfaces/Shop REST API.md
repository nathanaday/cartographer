---
id: ifc-r4t7uv
type: carto-interface
description: The REST API of shop-api, for products, carts, and orders.
tags: []
created: 2026-09-29T09:00:00
updated: 2026-09-29T09:00:00
kind: http
definition: github.com/myshop/myshop-api/server/openapi.yaml
stability: stable
schemas: ["[[Shop Data Schema]]"]
external: ""
---

## Purpose

Products, carts, and orders for the web front end.

## Wire

| | |
|---|---|
| Transport | HTTPS, JSON |
| Address | `https://api.myshop.com/v1`, public |
| Messages | Products: `GET /v1/products`, `GET /v1/products/{id}` (`a12/product.json`). Carts: `/v1/carts/{id}` and its lines (`a12/cart.json`). Orders: `POST /v1/orders`, `GET /v1/orders/{id}` (`a12/order.json`). |
| Auth | A bearer token on carts, orders, and the account. Products are public and hold no customer data. |
| Versioning | The path holds the major version. A breaking change makes `/v2`, and `/v1` stays for 90 days. |

`github.com/myshop/myshop-api/server/openapi.yaml` defines it in full. This document does not copy it.

## Attached services

```cartographer-attached
```

> [!example] What the plugin shows
> Attached: [[shop-api]] · [[shop-web]]

## Notes
