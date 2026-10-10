---
id: q-h6j8kl
type: carto-question
description: Does the currency belong to the order or to each order line?
tags: []
created: 2026-10-05T16:25:00
updated: 2026-10-06T09:10:00
status: answered
about: ["[[Shop Data Schema]]"]
holds: ""
asked_by: agent
session: ""
answered: 2026-10-06
defer: ""
---

## Question

Does the currency belong to the order or to each order line?

## Context

invoice-worker writes one invoice per order, and a cart with more than one currency is not possible today. Old messages on Order Events have no currency.

## Options

1. **Per order line** (recommended). A cart with more than one currency works later with no schema change.
2. **Per order.** Simpler now. A mixed cart later needs a change on every adopter.

## Answer

[chosen:: 1] [on:: 2026-10-06]

> Per line. We will want marketplace sellers in other countries. And make sure old orders still work, assume USD.

## Effects

- The draft clause DS-4 in [[Add currency to order lines]] gets the USD default.
- Requirement G-1 added to [[Add currency to order lines]].
