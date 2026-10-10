---
id: mod-a8d1fe
type: carto-module
description: Checks OAuth 2.0 bearer tokens for Go services.
tags: []
created: 2026-09-29T09:00:00
updated: 2026-09-29T09:00:00
repository: github.com/myshop/myshop-api
path: auth/
package: github.com/myshop/api/auth
adopts: []
compliance: 7/7
violated: 0
checked: 2026-10-01
---

## Purpose

Validates a bearer token against the identity provider's public keys and puts the customer id on the request context.

## Used by

```cartographer-children
```

> [!example] What the plugin shows
> [[shop-api]]

## Requirements

## Obligations

```cartographer-obligations
```

> [!example] What the plugin shows
> | Clause | Rule | From | Status | Result |
> |---|---|---|---|---|
> | [[Branch and CI Rules#CI-1]] | All work happens in a branch. Nobody commits … | [[MyShop]] | met · stale 1 commits, 7 days | [[2026-10-02 1500 Sweep#shop-auth]] |
> | [[Branch and CI Rules#CI-2]] | A merge to preview runs the full test … | [[MyShop]] | met · stale 1 commits, 7 days | [[2026-10-02 1500 Sweep#shop-auth]] |
> | [[Branch and CI Rules#CI-3]] | A branch merges to main only after its … | [[MyShop]] | met · stale 1 commits, 7 days | [[2026-10-02 1500 Sweep#shop-auth]] |
> | [[Security Policy#SEC-1]] | Code reads its secrets from Secret Manager at … | [[MyShop]] | met (none) | [[2026-10-02 1500 Sweep#shop-auth]] |
> | [[Security Policy#SEC-2]] | Every store of customer data uses the project's … | [[MyShop]] | met (none) | [[2026-10-02 1500 Sweep#shop-auth]] |
> | [[Security Policy#SEC-3]] | Every endpoint that the internet can reach and … | [[MyShop]] | met (none) | [[2026-10-02 1500 Sweep#shop-auth]] |
> | [[Security Policy#SEC-4]] | No log line holds a customer's name, email … | [[MyShop]] | met | [[2026-10-02 1500 Sweep#shop-auth]] |
>
> Compliance: met 7 / 7.

## Waivers

## Notes
