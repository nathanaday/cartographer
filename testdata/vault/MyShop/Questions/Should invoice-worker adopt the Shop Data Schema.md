---
id: q-1nvds1
type: carto-question
description: invoice-worker reads Order Events but does not adopt the Shop Data Schema. Should it?
tags: []
created: 2026-10-05T16:30:00
updated: 2026-10-05T16:30:00
status: open
about: ["[[invoice-worker]]", "[[Shop Data Schema]]"]
holds: lint L1 [[invoice-worker]]
asked_by: agent
session: ""
answered: ""
defer: ""
---

## Question

[[invoice-worker]] is attached to [[Order Events]], whose schema is [[Shop Data Schema]]. invoice-worker does not adopt that contract. Should it?

## Context

Lint L1 reported this when the agent ran lint on the draft of [[Add currency to order lines]]. invoice-worker reads more than the order id: `invoices/render.py:44` reads `line["price"]` and `line["quantity"]` from each order line. So it depends on the shape of the data, and DS-2 (prices in cents) already matters to it.

If it adopts the contract through the goal, DS-1, DS-2, DS-5, and DS-4 fall on it as pending obligations, and become active when the goal closes. An invoice in euros must show the currency, so T6 is real work in either case.

## Options

1. **Adopt** (recommended). The adoption is a map change of the goal, so invoice-worker gets 4 pending obligations: DS-1, DS-2, DS-5, and the new DS-4. They become active when the goal closes. The goal's obligations become 2 of 9.
   Change: add `adopt [[Shop Data Schema]] · [[invoice-worker]]` to the Map changes of [[Add currency to order lines]].
2. **Do not adopt, and do T6 anyway.** The map says nothing about the shape of the data that invoice-worker reads, and the next schema change will miss it again. Lint L1 stays until you waive it.
3. **Do not adopt, and drop T6.** Invoices in euros show no currency.

## Answer

## Effects
