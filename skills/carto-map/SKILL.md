---
name: carto-map
description: Map repositories into a Cartographer project, suggest intent from rules the code already enforces, turn a policy or schema document into a proposed contract, and refresh the map when code moved. Use for "map these repositories", "set up Cartographer for X", "add this repo to the map", "clauses from this document", and an unmapped finding.
---

# carto-map

The map holds what exists (services, modules, interfaces) and what must be true (systems and contracts). Code gives the first; the user declares the second. You propose both; the user approves.

## Set up a project

1. If no project exists, propose its document: `propose` with `project` set to the new folder (relative to the vault) and one `create` of `<Project>.md` at the folder's top, type `carto-project` (see references/templates.md), with its repositories (remotes such as `github.com/org/repo`), `default_branch`, `landing`, and `platform`. Obsidian cannot show the proposal before the project exists, so ask the user to approve it here, naming the change.
2. After the approval, run `carto setup --project <folder> --root <folder that holds the clones>` so this machine knows the clones. Report a remote with no clone.

## Map the code: one proposal for each repository

For each repository, read it and propose one change (`propose`, `create` operations) with:

- **Services**: each thing that runs: a server, a worker, a scheduled job, a pipeline step, a script that CI or a scheduler runs. Its `path` (the folder it owns), its `entry` list (the entry points and deploy units), `runtime` and `deploy` for the reader, its `interfaces`, `uses`, and `stores`.
- **Modules**: each package that services share.
- **Interfaces**: each connection between services, or with an external party: an API, a topic, a dataset, a bucket, a report. Fill the Wire table: Transport, Address, Messages, Auth, Delivery, Ordering, Versioning. Point `definition` to the file that defines it; do not copy it. An interface has no direction.
- A database that only one service uses is a `store` of that service, not an interface. A platform service any service may use (a secret store, logging) goes in the project's `platform`.

## Suggest intent

After the map, propose intent as separate changes, so the user accepts each on its own:

- **Systems**, from deploy groupings (the services that ship together or serve one product).
- **Contracts**, from rules the code already enforces: CI configuration and branch protection, linters, agent instruction files (CLAUDE.md, AGENTS.md), schema folders, decision records. Each clause cites the line it came from in its `evidence`. Write clauses broadly ("every endpoint that the internet can reach requires authentication"); checks find the parts.
- **Adoptions**: which entities adopt each contract. A schema contract is adopted by each service that reads or writes the data.

## Clauses from a document

Read the document. Propose one contract: one clause per rule that is true or false of code, numbered `<CODE>-1`, `-2`, …, with `verify` (how it is checked) and the source line in `evidence`. Leave out what code cannot satisfy (training, legal notices); say so in one line.

## An unmapped finding

Read the entry point. Propose either a new service (its own `path` and `entry`) or the entry point added to an existing service's `entry`, as the finding's options say, after the user chooses.

## Rules

- One fact in one place: each relation is stored once, in the field the templates name.
- Never invent a rule the user did not state or the code does not enforce. Suggested intent is marked as suggested in the change's title.
