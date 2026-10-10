---
id: fnd-sh2sto
type: carto-finding
description: Checkout hangs when recs-server is down, and SHOP-2 requires it to work.
tags: []
created: 2026-10-02T15:00:00
updated: 2026-10-02T15:10:00
kind: violation
entity: "[[Storefront]]"
clause: "[[Storefront#SHOP-2]]"
about: ""
key: ""
check: "[[2026-10-02 1500 Sweep#Storefront]]"
found: 2026-10-02
defer: ""
status: waived
contract: ""
resolution: ["[[Make checkout survive a recommendation outage]]"]
resolved: ""
---

## Finding

[[Storefront#SHOP-2]] requires that a customer can pay when recs-server is down. With recs-server stopped, checkout does not finish.

## Evidence

- `e2e/checkout-no-recs.spec.ts` times out after 30 s on preview.
- shop-api calls recs-server during checkout with no timeout, `server/checkout/recs.go:31`. The same call is the divergence [[shop-api calls Recs API without attaching it]].

## Options

1. **Add an upkeep task.**
2. **Start a goal** (recommended). The fix touches shop-api and the e2e tests in shop-web.
3. **Waive until a date, and draft a goal.**
   Change: add a waiver of [[Storefront#SHOP-2]] until 2026-11-15, and draft a goal that resolves this finding.

[chosen:: 3] [by:: user] [on:: 2026-10-02]

> Waive it for now, but draft the fix so it's ready when I have time.

## History

- 2026-10-02 15:00 opened by [[2026-10-02 1500 Sweep#Storefront]].
- 2026-10-02 15:10 the user chose option 3. Waived until 2026-11-15 ([[Storefront#^w1]]). The agent drafted [[Make checkout survive a recommendation outage]].
