---
id: sys-b4ck0f
type: carto-system
description: Stock and invoices for every paid order.
tags: []
created: 2026-09-29T09:00:00
updated: 2026-09-29T09:00:00
code: OPS
project: "[[MyShop]]"
adopts: []
compliance: 0/1
rollup: 9/18
violated: 1
---

## Purpose

Takes each paid order from the shop, reserves the stock, and writes the invoice that accounting imports.

## Requirements

### OPS-1
Each paid order gets exactly one invoice within 1 h.
[verify:: test] [scope:: whole]

## Services

```cartographer-children
```

> [!example] What the plugin shows
> | Service | Compliance | Violated |
> |---|---|---|
> | [[inventory-service]] | met 9 / 10 | 1 |
> | [[invoice-worker]] | met 0 / 7 | 0 |

## Obligations

```cartographer-obligations
```

> [!example] What the plugin shows
> | Clause | Rule | From | Status | Result |
> |---|---|---|---|---|
> | [[Back Office#OPS-1]] | Each paid order gets exactly one invoice within … | local | unverified | not yet checked |
>
> Compliance: met 0 / 1. Not yet checked: in the sweep queue.

## Waivers



## Notes
