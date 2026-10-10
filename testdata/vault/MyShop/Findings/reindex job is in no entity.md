---
id: fnd-r31ndx
type: carto-finding
description: A Cloud Run job, reindex, starts from an entry point in myshop-api that no entity lists.
tags: []
created: 2026-10-08T02:00:00
updated: 2026-10-08T02:00:00
kind: unmapped
entity: ""
clause: ""
about: ""
key: myshop-api:server/cmd/reindex/main.go
check: "[[2026-10-08 0200 Sweep]]"
found: 2026-10-08
defer: ""
status: open
contract: ""
resolution: []
resolved: ""
---

## Finding

Commit 9f8e7d6 added `server/cmd/reindex/main.go` to myshop-api, and `myshop-infra/jobs.tf:8` deploys it as the Cloud Run job `reindex`. No entity lists it in `entry`.

shop-api's `path` is `server/`, so it owns the file, and the before-hook gives an agent shop-api's rules when it edits it. But the job runs on its own schedule and with its own identity, and no check covers it as something that runs.

## Evidence

- `server/cmd/reindex/main.go`: rebuilds the product search index from Cloud SQL `shop-main` each night.
- `myshop-infra/jobs.tf:8`: the job and its schedule.

## Options

1. **Add a service reindex-job to Storefront** (recommended). Its `path` is `server/cmd/reindex/`, so it owns its own files. It reads shop-main, which is a store of shop-api only today, so the agent will also propose an interface for the shared database.
   Change: propose the service reindex-job in [[Storefront]].
2. **Add it to the `entry` of shop-api.** shop-api's rules then cover the job too.
   Change: propose `server/cmd/reindex/main.go` in the `entry` of [[shop-api]].
3. **Waive.**
   Change: add a waiver of this finding to [[MyShop]].

## History

- 2026-10-08 02:00 opened by the coverage check of [[2026-10-08 0200 Sweep]].
