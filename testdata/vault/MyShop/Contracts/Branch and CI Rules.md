---
id: con-c1rul5
type: carto-contract
description: How code reaches main in every MyShop repository.
tags: []
created: 2026-09-29T09:00:00
updated: 2026-09-29T09:00:00
code: CI
kind: process
status: active
---

## Purpose

No change reaches production without passing the full test suite.

## Clauses

### CI-1
All work happens in a branch. Nobody commits to `main` directly.
[verify:: ci] [evidence:: branch protection on main]

### CI-2
A merge to `preview` runs the full test suite.
[verify:: ci]

### CI-3
A branch merges to `main` only after its CI run on `preview` passes.
[verify:: ci] [evidence:: required status check preview-tests]

## Adopters

```cartographer-adopters
```

> [!example] What the plugin shows
> | Entity | CI-1 | CI-2 | CI-3 |
> |---|---|---|---|
> | [[inventory-service]] | met | met | met |
> | [[invoice-worker]] | unverified | unverified | unverified |
> | [[myshop-common]] | met | met | met |
> | [[recs-server]] | met | met | met |
> | [[recs-trainer]] | met | met | met |
> | [[shop-api]] | met | met | met |
> | [[shop-auth]] | met · stale | met · stale | met · stale |
> | [[shop-web]] | met | met | met |
>
> Adopted by [[MyShop]]. **Adopt for…**

## History
