---
id: con-s3cp0l
type: carto-contract
description: How MyShop keeps secrets, customer data, and endpoints safe.
tags: []
created: 2026-09-29T09:00:00
updated: 2026-09-29T09:00:00
code: SEC
kind: security
status: active
---

## Purpose

The rules that every service and module obeys, whatever it does. They are broad on purpose. A check finds the parts of each entity that each rule applies to.

## Clauses

### SEC-1
Code reads its secrets from Secret Manager at run time. No secret is in a repository, an env file, or an image.


### SEC-2
Every store of customer data uses the project's encryption key (CMEK).


### SEC-3
Every endpoint that the internet can reach and that reads or changes customer data requires authentication.


### SEC-4
No log line holds a customer's name, email address, or street address.


## Adopters

```cartographer-adopters
```

> [!example] What the plugin shows
> | Entity | SEC-1 | SEC-2 | SEC-3 | SEC-4 |
> |---|---|---|---|---|
> | [[inventory-service]] | met | met | met | met |
> | [[invoice-worker]] | unverified | unverified | unverified | unverified |
> | [[myshop-common]] | met | met (none) | met (none) | met |
> | [[recs-server]] | met (none) | met (none) | met (none) | met |
> | [[recs-trainer]] | met (none) | met | met (none) | met |
> | [[shop-api]] | met | met | met | met |
> | [[shop-auth]] | met (none) | met (none) | met (none) | met |
> | [[shop-web]] | met | met (none) | met | met |
>
> Adopted by [[MyShop]]. **Adopt for…**

## History
