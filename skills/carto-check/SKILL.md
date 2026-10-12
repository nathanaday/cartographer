---
name: carto-check
description: Check Cartographer obligations against the code. Use for "check shop-api", "is the shop compliant with the Security Policy", "run a sweep", "check what I changed", re-checking stale results, and after work that changed code under an obligation. It sends a read-only checker for each entity and writes the results with the record tool.
---

# carto-check

A check verifies the obligations of one entity at one commit. A sweep checks the queued obligations of many entities, and the coverage of each repository.

## Check one entity

1. Call `map` with the entity. Note its repository, path, the obligations to check, and for a service its interfaces, uses, and stores.
   - After your own work: check every obligation whose clause your change could touch, and every pending obligation of the goal you work for.
   - On request: all obligations.
   - In a sweep: those that are unverified or stale.
2. Find the commit: `git -C <clone> rev-parse HEAD` and the branch. Results on a goal branch count toward the goal only, until the branch lands. Ask the user for the clone only when `carto setup` has none.
3. Send the **checker** agent (read-only). Give it the entity, the clone, the commit and ref, each obligation with its clause link, revision, statement, and `verify` value, the declared dependencies, and the sweep name when there is one. A system check covers the system's own requirements and names a commit for each repository of its services.
4. Call `record` with `action: check` and the checker's report. If `record` refuses a row, fix the row with the checker, do not drop it.
5. Report in one line: met, violated, and inconclusive counts, and the findings opened or resolved, with links.

## A sweep

1. `record` with `action: start_sweep`. Keep the name.
2. Call `attention` and read the sweep queue in Health. Order the entities: those an active goal affects, then those with recent commits, then the rest.
3. For each entity, check as above with `sweep` set to the sweep's name. Run several checkers in parallel when they read different clones.
4. Coverage: for each repository of the project, list what runs outside a developer's machine: the entry points of what ships or deploys (a main package or script that a package, an image, a unit, or a job runs), the deploy units (an image that is deployed, a service or job that infrastructure code defines), and the scripts that CI or a scheduler runs. A tool a person runs by hand, a build file, a local development setup (such as a compose file for local services), and the infrastructure code itself are not entry points. Mark each `mapped` when an entity lists it in `entry`, else `unmapped`, with the evidence. Leave out what the project's Waivers already accept.
5. `record` with `action: finish_sweep`, the sweep name, the coverage rows, and one note for what the sweep did not reach.

## Rules

- A checker is strict: a clause is met only on evidence it saw.
- Never mark a result by hand, and never edit a check or a finding. A check is never changed after it is written.
- An inconclusive result is fine when the reason is real. If it needs the user, ask with `work` (`action: ask`), so the obligation waits for the answer instead of the next sweep.
