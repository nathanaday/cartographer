---
id: mod-c0mm0n
type: carto-module
description: Shared Python code for secrets, logging, and Pub/Sub.
tags: []
created: 2026-09-29T09:00:00
updated: 2026-09-29T09:00:00
repository: github.com/myshop/myshop-common
path: myshop_common/
package: myshop-common (private package index)
adopts: []
compliance: 7/7
violated: 0
checked: 2026-10-02
---

## Purpose

Gives each Python service one way to read secrets, write logs with customer data removed, and publish or read Pub/Sub messages.

## Used by

```cartographer-children
```

> [!example] What the plugin shows
> [[inventory-service]] · [[invoice-worker]] · [[recs-server]] · [[recs-trainer]]

## Requirements

## Obligations

```cartographer-obligations
```

> [!example] What the plugin shows
> | Clause | Rule | From | Status | Result |
> |---|---|---|---|---|
> | [[Branch and CI Rules#CI-1]] | All work happens in a branch. Nobody commits … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#myshop-common]] |
> | [[Branch and CI Rules#CI-2]] | A merge to preview runs the full test … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#myshop-common]] |
> | [[Branch and CI Rules#CI-3]] | A branch merges to main only after its … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#myshop-common]] |
> | [[Security Policy#SEC-1]] | Code reads its secrets from Secret Manager at … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#myshop-common]] |
> | [[Security Policy#SEC-2]] | Every store of customer data uses the project's … | [[MyShop]] | met (none) | [[2026-10-02 1500 Sweep#myshop-common]] |
> | [[Security Policy#SEC-3]] | Every endpoint that the internet can reach and … | [[MyShop]] | met (none) | [[2026-10-02 1500 Sweep#myshop-common]] |
> | [[Security Policy#SEC-4]] | No log line holds a customer's name, email … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#myshop-common]] |
>
> Compliance: met 7 / 7.

## Waivers

## Notes
