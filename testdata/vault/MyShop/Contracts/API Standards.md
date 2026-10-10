---
id: con-ap1std
type: carto-contract
description: Speed, authentication, and error format of every HTTP API that MyShop serves.
tags: []
created: 2026-09-29T09:00:00
updated: 2026-09-29T09:00:00
code: API
kind: performance
status: active
---

## Purpose

Every HTTP API in MyShop behaves the same way for its callers.

## Clauses

### API-1
The server handles 95 % of requests in 25 ms or less, from the time it receives a request to the time it sends the response.
[verify:: metric]

### API-2
The API accepts OAuth 2.0 bearer tokens and validates them with [[shop-auth]] or with the identity provider's library.


### API-3
An error response is a problem document as RFC 9457 defines it.
[verify:: test]

## Adopters

```cartographer-adopters
```

> [!example] What the plugin shows
> | Entity | API-1 | API-2 | API-3 |
> |---|---|---|---|
> | [[recs-server]] | met | met | met |
> | [[shop-api]] | unverified | met · stale | met |
>
> Adopted by [[recs-server]] · [[shop-api]]. **Adopt for…**

## History
