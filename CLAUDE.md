# cartographer

Cartographer keeps a literal map of a software ecosystem as linked Markdown documents in an
Obsidian vault: systems, services, modules, interfaces, contracts with numbered clauses, the
obligations they create, checks with evidence, findings, and goals with tasks.

This repository holds all three parts:

- **The core**: the Go module `github.com/nathanaday/cartographer`, binary `carto`. It is the
  one library that computes everything (`internal/core`), renders it (`internal/view`), and
  serves the CLI (`internal/cli`), the MCP tools, the hooks, and the runner.
- **The agent plugin**: `.claude-plugin/`, `skills/`, `agents/`, `hooks/`, `bin/carto`.
- **The Obsidian plugin**: `obsidian/`. It runs the CLI for every read and write, so the
  rules live in one place.

## Sources of truth

| Thing | Location |
|---|---|
| The specification | `~/Vaults/SoftwareProjects/cartographer-design/Specification.md` |
| The worked example | `~/Vaults/SoftwareProjects/cartographer-design/MyShop/` and `Walkthrough.md` |
| The reference implementation of the rules | `testdata/prototype/carto.py` (Python), with `simulation.yaml` |
| The test vault | `testdata/vault/` (a copy of the example, as the prototype rendered it) |

The specification is the design. When the code departs from it, the reason is in
`docs/architecture.md`.

## Rules for working here

- **The golden test is the contract of the core.** `internal/view/golden_test.go` strips every
  rendered callout and tool field from the test vault, renders them again with the Go core,
  and requires the result to be byte for byte what the prototype wrote. A change to a rule
  changes the prototype, the example in the vault, and the Go core together.
- **The core reads; it never writes.** `internal/core` takes documents and an `Env` and computes.
  Every write goes through one `vault.Writer` and ends in `app.Commit`: one act, one commit.
- **The plugin holds no rule.** `obsidian/` decides what to show and which `carto` command a
  button runs. A rule that the plugin needs goes into the core and comes out as JSON.
- **`Env` hides the world.** `core.Sim` reads a simulation file in place of git, the claims,
  and the clock. The live environment reads real git and the vault's `.carto/` files.
- Prose in docs, skills, and messages follows the user's global writing guide: short
  sentences, active voice, one term for one thing.

## Commands

```
make test       # go vet and go test
make build      # build/carto
make install    # ~/.cartographer/bin/carto, where the plugins find it
make plugin     # obsidian/dist
make e2e        # the Obsidian plugin in a real, separate Obsidian (macOS)
go run ./cmd/carto render testdata/vault/MyShop --sim testdata/prototype/simulation.yaml --examples
```

`docs/architecture.md` holds the packages, the write path, Undo, and what is not built yet.
