---
id: q-ap1prf
type: carto-question
description: API-1 on shop-api has no evidence. Which proof do we use?
tags: []
created: 2026-10-01T17:40:00
updated: 2026-10-01T17:40:00
status: open
about: ["[[API Standards]]", "[[shop-api]]"]
holds: ""
asked_by: agent
session: ""
answered: ""
defer: ""
---

## Question

[[API Standards#API-1]] limits handling time on the server to 25 ms for 95 % of requests. shop-api has no measure of that time. How do we prove it?

## Context

The request latency of Cloud Run includes the network and the queue, so it cannot prove API-1. recs-server proves API-1 with a histogram of handling time, written by OpenTelemetry. shop-api has neither a histogram nor a load test.

## Options

1. **A metric, as recs-server does** (recommended). Add the OpenTelemetry HTTP histogram to shop-api and a panel to the latency dashboard.
   Change: add a task for [[shop-api]] to [[Upkeep]], with this question as its source.
2. **A load test in preview CI.** k6 against preview. It proves the limit before each release, but not under real traffic.
   Change: add a task for [[shop-api]] to [[Upkeep]], with this question as its source.
3. **Waive API-1 on shop-api** until a date you choose.
   Change: add a waiver of [[API Standards#API-1]] to [[shop-api]].

## Answer

## Effects
