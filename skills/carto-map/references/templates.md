# Document templates

Each Cartographer document is Markdown with flat frontmatter. Write each one as the `text` of a `create` operation, at the path shown (relative to the project folder). Use a new id: a type prefix and six random lowercase letters and digits. Leave tool fields as shown; the tool fills them. A text value that contains `: ` or starts with `[`, `{`, `"`, or `#` is in double quotes. Relations are lists of quoted links.

## Project · `<Project>.md`

```markdown
---
id: prj-xxxxxx
type: carto-project
description: One sentence.
tags: []
created: 2026-10-09T10:00:00
updated: 2026-10-09T10:00:00
adopts: []
repositories: ["github.com/org/repo-a", "github.com/org/repo-b"]
platform: ["Secret Manager", "Cloud Logging"]
default_branch: main
landing: user
ci_jobs: []
max_agents: 3
task_budget: 120
stale_after: 30
nudge_after: 14
canvas: "[[<Project>.canvas]]"
rollup: ""
violated: 0
---

## Purpose

## Since your last visit

```cartographer-since
```

## Attention

```cartographer-attention
```

## Goals

```cartographer-goals
```

## Health

```cartographer-health
```

## Waivers
```

## System · `Systems/<Name>.md`

```markdown
---
id: sys-xxxxxx
type: carto-system
description: One sentence.
tags: []
created: …
updated: …
code: SHOP
project: "[[<Project>]]"
adopts: []
compliance: ""
rollup: ""
violated: 0
---

## Purpose

## Requirements

### SHOP-1
A statement that is true or false of the system.
[verify:: test] [evidence:: e2e/checkout.spec.ts] [scope:: whole]

## Services

```cartographer-children
```

## Obligations

```cartographer-obligations
```

## Waivers

## Notes
```

## Service · `Services/<name>.md`

```markdown
---
id: svc-xxxxxx
type: carto-service
description: One sentence.
tags: []
created: …
updated: …
systems: ["[[Storefront]]"]
repository: github.com/org/repo
path: server/
entry: ["server/cmd/api/main.go"]
runtime: Go 1.23
deploy: Cloud Run service api
interfaces: ["[[Shop REST API]]"]
uses: []
stores: ["Cloud SQL main"]
adopts: []
compliance: ""
violated: 0
checked: ""
---

## Purpose

## Requirements

## Obligations

```cartographer-obligations
```

## Waivers

## Work

```cartographer-work
```

## Notes
```

## Module · `Modules/<name>.md`

Like a service, with `repository`, `path`, `package`, and `adopts`; no `systems`, `entry`, `interfaces`, `uses`, `stores`. Body: Purpose · Used by (` ```cartographer-children``` `) · Requirements · Obligations · Waivers · Notes.

## Interface · `Interfaces/<Name>.md`

```markdown
---
id: ifc-xxxxxx
type: carto-interface
description: One sentence.
tags: []
created: …
updated: …
kind: event
definition: github.com/org/schemas/order.json
stability: stable
schemas: ["[[Shop Data Schema]]"]
external: ""
---

## Purpose

## Wire

| | |
|---|---|
| Transport | Google Cloud Pub/Sub |
| Address | topic `orders.paid` |
| Messages | `OrderPaid`: one for each paid order |
| Delivery | At least once |
| Versioning | Fields are only added |

## Attached services

```cartographer-attached
```

## Notes
```

## Contract · `Contracts/<Name>.md`

```markdown
---
id: con-xxxxxx
type: carto-contract
description: One sentence.
tags: []
created: …
updated: …
code: DS
kind: schema
definition: github.com/org/schemas/a12/
status: active
---

## Purpose

## Clauses

### DS-1
One rule that is true or false of the code.
[verify:: test] [evidence:: where the proof lives]

## Adopters

```cartographer-adopters
```

## History
```

Clause fields are written only when they differ from the default: `verify` (default `agent`), `scope` (default `each`), `rev` (default 1), `state` (default `active`).

## Goal · `Goals/<Title>.md`

```markdown
---
id: goal-xxxxxx
type: carto-goal
description: One sentence.
tags: []
created: …
updated: …
status: draft
priority: medium
after: []
defer: ""
branch: goal/short-name
resolves: []
affects: []
progress: ""
tasks: ""
landed: ""
opened: ""
closed: ""
---

## Intent

> The user's own words.

## Map changes

- **add** [[Shop Data Schema#DS-new]] · A new rule. [verify:: test]

## Requirements

### G-1
A statement for this goal only.
[verify:: test] [on:: [[shop-api]]]

## Decisions

## Out of scope

## Tasks

- [ ] T1 [[shop-api]] What to do [for:: DS-new] ^t1

## Task details

## Progress

```cartographer-progress
```

## Log
```
