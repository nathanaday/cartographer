---
name: carto-goal
description: Draft, review, run, land, and close a Cartographer goal, a planned change to the map that takes days or weeks across repositories. Use for "I want to …" requests that change rules, interfaces, or services; "work on the currency goal"; "resume the goal"; and "is the goal done". A small change in one repository needs no goal.
---

# carto-goal

A goal is a planned change to the map. It is done when its work has landed and the map is true again. The user approves it once; then the work runs without stops.

## Draft

1. Quote the user's intent in their own words.
2. Call `impact` for each contract, clause, or interface the intent touches. The affected services come from the map, not from memory.
3. Write the goal document with `propose`, a `create` operation in `Goals/` (see carto-map's references/templates.md for the format), `status: draft`, and a branch name `goal/<short-name>`. A draft applies at once, because the draft is itself the proposal: it changes nothing on the map until the user approves it. Revise a draft by editing its document directly while it stays a draft.
   - **Map changes**, one act per line: `- **add** [[Contract#XX-new]] · statement [verify:: test]`, `- **revise** [[Contract#XX-n]] · new statement`, `- **retire** [[Contract#XX-n]]`, `- **adopt** [[Contract]] · [[entity]]`, `- **attach** [[Interface]] · [[service]]`, `- **use** [[module]] · [[service]]`, `- **join** [[system]] · [[service]]`.
   - **Requirements** for what does not belong on the map for good: `### G-1`, a statement, `[verify:: test] [on:: [[entity]]]`. A goal with no clause change still has one.
   - **Decisions**, **Out of scope**, and **Tasks**: one task per entity and step, `- [ ] T1 [[entity]] text [for:: XX-new] [after:: T…] ^t1`. A definition change (a schema file) comes first.
4. Call `lint` with the goal. Ask the user (with `work`, `action: ask`) about any gap that would change the scope, and block the tasks it touches.
5. Send the **reviewer** agent in `draft` mode. Add its findings as tasks, or as questions when they touch scope.
6. Tell the user in two lines what the goal does and that it waits for their approval. They approve the draft goal in Obsidian, or in chat; in chat, call `propose` with a `goal.approve` operation on the goal, titled with the goal's own title, so that the user's words name it, then `approve`.

## Run

The runner (`carto run`, or **Run now** in Obsidian) starts the ready tasks of an approved goal, each in its own session. In a chat, after the user approves a goal, say that the runner starts its tasks; do a task here only when the user asks you to. The runner sessions and the user's request both follow these steps, without stops, for each ready task (the `attention` tool lists them):

1. Work in a worktree of the goal's branch. The runner makes it for its sessions under `~/.cartographer/worktrees/`. In a chat, add your own outside the clone's folder: `git -C <clone> worktree add ~/.cartographer/worktrees/<repository>/<branch with - for />-chat -b <branch>` (or without `-b` when the branch exists). While it holds the branch, the runner waits. Never switch the user's own checkout.
2. Do the task. Commit with a message that names the goal and the task.
3. Check the entity's affected obligations on the branch (carto-check). Fix and check again until they are met.
4. `work` with `action: done`, the goal, the task, and the commit.
5. A failure you cannot fix: `work` with `action: attempt` and the reason. After two attempts the task is stuck and waits for the user.
6. A question that would change the scope: `work` with `action: ask` and `blocks`, then go on with every task it does not block.

## Land and close

1. When every task is done: rebase each branch on the default branch, re-check the goal's pending obligations whose results are stale, and send the **reviewer** in `finished` mode. If the goal's Log has a "not ready to land" line, give the reviewer the commit it names: the review then covers only the commits since. Each blocking finding becomes a task, or a question when it touches scope. Each follow-up becomes one line under "After the goal lands:" in Task details, not a task and not a question. Log "ready to land" when nothing blocks, else "not ready to land" with the branch's commit.
2. Land by the project's `landing` policy: under `agent`, open a pull request in each repository and merge it when its checks pass; under `user`, open the pull requests and tell the user the goal waits for their Land decision. Definition repositories land first. Record each merge with `work` (`action: log`) as `landed <repository> <merge commit>`. A project with `pull_requests: false` has no forge: do not push or open pull requests; under `agent`, land with `carto land <goal>`, which merges each branch in its clone and writes the `landed` lines.
3. Check every obligation of the goal at the default branch. When all are met, call `work` with `action: close`. The tool refuses while anything is missing and says what.

End each session with one or two lines: what moved, what waits for the user, and the goal's link.
