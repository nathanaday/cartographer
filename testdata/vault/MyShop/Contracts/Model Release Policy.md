---
id: con-mr3l5p
type: carto-contract
description: When a new recommendation model may replace the live one, and how to take it back.
tags: []
created: 2026-09-30T16:30:00
updated: 2026-09-30T16:30:00
code: MRP
kind: process
status: active
---

## Purpose

A bad model must never reach customers silently, and a bad release must be quick to undo.

## Clauses

### MRP-1
Each model version records its training data snapshot, its code commit, and its metrics on the holdout set.


### MRP-2
A new model becomes live only if its NDCG@10 on the holdout set is at least the live model's.
[verify:: test]

### MRP-3
The previous model is live again within 5 min of the decision to roll back.
[verify:: review] [scope:: whole]

## Adopters

```cartographer-adopters
```

> [!example] What the plugin shows
> | Entity | MRP-1 | MRP-2 | MRP-3 |
> |---|---|---|---|
> | [[Recommendations]] |  |  | unverified |
> | [[recs-server]] | met | met |  |
> | [[recs-trainer]] | met | met |  |
>
> Adopted by [[Recommendations]]. **Adopt for…**

## History
