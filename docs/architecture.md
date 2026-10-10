# Architecture

This document records how the code carries out the specification, and why. The
specification itself is in the SoftwareProjects vault (see `CLAUDE.md`).

## One core, three surfaces

The specification asks for one core library that the Obsidian plugin and the agent tools
both use. The core is the Go binary `carto`:

| Surface | How it reaches the core |
|---|---|
| Obsidian plugin (`obsidian/`) | runs `carto … --json` for every block, button, command, and canvas edit |
| Agent plugin (repository root) | the MCP server (`carto mcp`) and the hooks (`carto hook <name>`) |
| Runner, CI, a person | the CLI (`carto run`, `carto record --inbox`, `carto check`) |

Go, not TypeScript, for the core: the hooks run on every tool call and must start fast; the
runner supervises processes, worktrees, and git; and Almagest has the same shape, so
distribution and habits carry over. The Obsidian plugin holds no rule of the map. It decides
only what to show and which command a button runs.

## Packages

| Package | Holds |
|---|---|
| `internal/md` | frontmatter, sections, wikilinks, inline fields, tables; text edits |
| `internal/core` | the index of a project and every computed fact (reads only) |
| `internal/view` | rendered blocks, Attention as structured items, tool fields, canvases |
| `internal/vault` | the one vault writer: the lock, files, and one git commit for each write |
| `internal/change` | change documents, declared operations, approval, the user's acts, and Undo |
| `internal/work` | observed work: tasks, questions, choices, defers, goal logs |
| `internal/record` | checks and sweeps: validation of a report, findings, coverage |
| `internal/refresh` | stages the tool fields and the canvases that follow from a write |
| `internal/gesture` | reads an edit of the project canvas as declared operations |
| `internal/templates` | new entity documents for the side panel |
| `internal/consent` | the user's recent prompts, which the approval tools require |
| `internal/hooks` | the agent hooks: the prompt, the rules before an edit, the guard, the stop |
| `internal/mcpserver` | the MCP tools of the agent plugin |
| `internal/runner` | `carto run`: the queue, worktrees, claims, the lease, session logs |
| `internal/land` | landing through pull requests, and the `landed` lines |
| `internal/live` | the live environment: real git, claims, and Mark seen |
| `internal/app` | joins settings, environment, and core; `app.Commit` finishes every write |
| `internal/cli` | the commands |

## The environment

`core.Env` is everything the core needs beyond the documents: the clock, git, the claims,
and the user's last visit.

- `core.Sim` reads a simulation file. The test vault and the design example use it. It keeps
  the prototype's approximations: commits are ordered by date, and a commit counts as on the
  default branch when it has no ref or its goal's Log records that it landed.
- `live.Env` reads real git (reachability with `merge-base --is-ancestor`, staleness with
  `git log <commit>..<default> -- <path>`), the claim files in `.carto/claims/`, and
  `.carto/seen.json`.

## Writes

Every write goes through one `vault.Writer`. It takes the vault's lock at its first read and
holds it until the commit, so a read, its change, and its write are one step. A write ends in
`app.Commit`, which writes the files, stages the tool fields and canvases that follow from
them, and makes **one commit**. One act is one commit: its operations, its record in the
change document, and the refresh.

The vault's git can be busy with another program (an editor, a sync plugin). Each git call
waits up to five seconds for `index.lock` to go.

## Undo

An act stores its **base** (the vault's commit before it) and, for each file its operations
wrote, a hash of the file's text after the act. Undo restores those files from the base, but
only while each still has that text. The test is on content, not on commits, so it needs no
special case for refreshes or for acts that were undone before. Acts in the same second are
ordered by their place in one change document, or else by the ancestry of their bases.

Before an act, the writer commits any edits by hand of the files the act touches. Undo then
restores the hand edits too, and does not lose them.

## The user's acts and the agent's

| Who | Declared state | How it applies |
|---|---|---|
| The user, in Obsidian or the CLI | canvas gestures, New, answers, choices, Approve | at once, as one act in the hour's "Your edits" document |
| An agent | `propose` | a proposed change, applied when the user approves |
| An agent, for a user's choice | `propose` with `because` | at once, as the user's act, once |

A choice whose option carries a `Change:` gets `[apply:: pending]` on its choice line. The
runner's queue starts an **apply** job for each pending choice, so a decision made in
Obsidian is carried out without the user opening a session. What carries it out replaces
`pending` with a link to the change document or the new upkeep task.

A decision on a lint result becomes an answered question that `holds` the lint, so it follows
the same path.

## The canvas

`gesture.Read` reads the relations a canvas shows from geometry and labels alone: a group is
classified by its label and its parent group, a card belongs to the innermost group that
holds its center, and a service belongs to the system groups that hold it. It runs the same
function on the canvas the core generates and on the canvas the user saved; the difference
is the act. No node ids are needed, so copy and paste work, and the layout cannot leak into
the meaning. Everything else the user changed goes back at the next layout, with one note
for each kind.

## The runner

The queue: closing goals, pending choices to apply, landings, upkeep tasks without a due
date, goal tasks by priority, upkeep tasks with a due date, then the sweep. Each task runs in
its own worktree under `~/.cartographer/worktrees/`, with a claim, a session log, and a time
budget. Claims and the lease are versioned (Specification 6.2), so each is a commit. The
lease lasts one hour and is renewed every twenty minutes, so a long run makes few commits. A
task that is not done when its session ends counts as a failed attempt; two make it stuck.

## The agent plugin

| File | Does |
|---|---|
| `.claude-plugin/plugin.json` | the manifest |
| `.mcp.json` | the MCP server: `bin/carto mcp` |
| `hooks/hooks.json` | prompt, before (the rules), guard, stop |
| `bin/carto` | runs `$CARTO_BIN`, else `~/.cartographer/bin/carto`, else `build/carto`; a hook with no binary passes quietly |
| `skills/` | `carto` (the front door), `carto-map`, `carto-check`, `carto-goal` |
| `agents/` | `checker` and `reviewer`, both read-only |

Approval needs the user's own words: the prompt hook records each prompt, and `approve`,
`undo`, `answer`, `choose`, and `defer` accept only words the user wrote in the last 30
minutes. The guard refuses an agent's direct write to declared state, computed fields,
rendered blocks, and checks.

## Not built yet

- Rename, move to another repository, and Remove (Specification 8.3).
- Drop of an active goal (pending clauses struck through). A draft goal can be dropped.
- The **Confirm** line for a change that no surface wrote.
- Pull before and push after each commit when the vault has a remote (Specification 6.4).
- The order of groups that the user sets on the canvas; the layout puts them back.
- System canvases for a project with more than 12 services.
