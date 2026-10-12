---
name: carto
description: Cartographer's front door. Use for "what waits for me", "status", "what changed", "where does this goal stand", answering a question or a finding the user decided in chat, approving or rejecting a proposed change in the user's words, and routing any other Cartographer request. Mapping repositories is carto-map; checks and sweeps are carto-check; goals are carto-goal.
---

# carto

Cartographer keeps a map of a software ecosystem in an Obsidian vault: systems, services, modules, interfaces, contracts with numbered clauses, the obligations those clauses create, checks with evidence, findings, and goals with tasks. The user makes decisions; agents do the work and keep the map true.

## Route the request

| The user wants | Do |
|---|---|
| What waits for them, what changed, the state of the project | Call `attention`. Report in a few lines: what changed, then the decisions that wait, then the goals. Link the documents. |
| One entity, contract, or interface | Call `map`. |
| What a change would touch | Call `impact` with the contract, clause (`Contract#ID`), interface, or module. |
| To map repositories, or turn a document into clauses | Use **carto-map**. |
| A check, a sweep, or "is X compliant" | Use **carto-check**. |
| A new goal, or work on a goal | Use **carto-goal**. |
| A small change in a repository | Do it. No goal and no task. A hook gives you the entity's rules before your first edit; check the entity when you are done (carto-check). |

## The user decides in chat

These are the user's acts. The tools check the user's own recent prompt, so quote it exactly.

- **Approve or reject a proposed change**: call `approve` with the change's id or name (and `reject: true` to reject). The user's prompt must say to approve (or reject) and name the change (every word of its title, its id, its name, or a document it creates), or say "all" for the changes proposed before it. Approve only what the user's words cover.
- **Answer a question**: call `work` with `action: answer`, `doc`, `chosen` (the option number), and `words` (the user's words). Then, if the option carries a `Change:`, call `propose` with `because` set to the question and the operations that carry out that change. It applies at once, once, as the user's act.
- **Choose a finding's option**: `work` with `action: choose`, then `propose` with `because` for its change.
- **Defer**: `work` with `action: defer`, `doc`, `until`, and `words`.
- **Undo**: call `undo` only when the user asked to undo.

## Rules

- Declared state changes only through `propose`: the fields `systems`, `adopts`, `interfaces`, `uses`, `stores`, `platform`, and `entry`, and the sections Clauses, Requirements, Waivers, and Map changes. Checks and findings change through `record`; tasks, questions, and logs through `work`. The guard refuses a direct write to any of these, and to tool fields and rendered blocks.
- To correct a proposed change that waits for the user, propose the corrected one with `replaces` set to the old change. The tool withdraws the old one, so the user decides on one. Do not ask the user to reject the old one.
- Prose and fields for the reader (Purpose, Notes, `description`, `runtime`, `deploy`, `package`) you edit directly with Edit.
- You never run `carto approve`, `undo`, `act`, `answer`, `choose`, `defer`, or `seen` in a shell. Those are the user's acts in Obsidian.
- End with one or two lines that say what is done, and link the documents that hold the detail. Do not report the vault's version control.
