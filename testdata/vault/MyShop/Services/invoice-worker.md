---
id: svc-1nv01c
type: carto-service
description: Writes a PDF invoice for each paid order.
tags: []
created: 2026-09-29T09:00:00
updated: 2026-09-29T09:00:00
systems: ["[[Back Office]]"]
repository: github.com/myshop/myshop-inventory
path: invoices/
entry: ["invoices/worker.py"]
runtime: Python 3.12
deploy: Cloud Run job invoice-worker, started by a Pub/Sub push
interfaces: ["[[Invoice PDFs]]", "[[Order Events]]"]
uses: ["[[myshop-common]]"]
stores: []
adopts: []
compliance: 0/7
violated: 0
checked: ""
---

## Purpose

Reads each paid order from Order Events, renders its invoice from an HTML template, and stores the PDF. Accounting imports the PDFs each night.

## Requirements

## Obligations

```cartographer-obligations
```

> [!example] What the plugin shows
> | Clause | Rule | From | Status | Result |
> |---|---|---|---|---|
> | [[Branch and CI Rules#CI-1]] | All work happens in a branch. Nobody commits … | [[MyShop]] | unverified | not yet checked |
> | [[Branch and CI Rules#CI-2]] | A merge to preview runs the full test … | [[MyShop]] | unverified | not yet checked |
> | [[Branch and CI Rules#CI-3]] | A branch merges to main only after its … | [[MyShop]] | unverified | not yet checked |
> | [[Security Policy#SEC-1]] | Code reads its secrets from Secret Manager at … | [[MyShop]] | unverified | not yet checked |
> | [[Security Policy#SEC-2]] | Every store of customer data uses the project's … | [[MyShop]] | unverified | not yet checked |
> | [[Security Policy#SEC-3]] | Every endpoint that the internet can reach and … | [[MyShop]] | unverified | not yet checked |
> | [[Security Policy#SEC-4]] | No log line holds a customer's name, email … | [[MyShop]] | unverified | not yet checked |
>
> Compliance: met 0 / 7. Not yet checked: in the sweep queue.
>
> Lint: [[invoice-worker]] is attached to [[Order Events]], whose schema is [[Shop Data Schema]], but does not adopt it.

## Waivers

## Work

```cartographer-work
```

> [!example] What the plugin shows
> - T6 of [[Add currency to order lines]] (active): Print the currency on each invoice line. Waits on [[Should invoice-worker adopt the Shop Data Schema]].
> - Question for you: [[Should invoice-worker adopt the Shop Data Schema]].

## Notes
