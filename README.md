# Cartographer

Cartographer keeps a map of your software ecosystem in an Obsidian vault: systems, services,
modules, interfaces, and contracts with numbered clauses. Checks record whether the code obeys
each clause, with evidence. Findings say what is wrong. Goals change the map, and agents do
the work.

You make the decisions. Agents do the work and keep the map true.

## What you get

- **A dashboard** in the project document: what changed since your last visit, the
  decisions that wait for you (with their options as buttons), the goals, and the health
  of the map.
- **A canvas** of the structure, with no lines. Drop a contract card into a service's group
  to adopt it; move a service group into another system; Undo takes it back.
- **Agents that know the rules.** Before an agent first edits a file, it gets the rules of
  the service that owns the file. A guard refuses its direct writes to the map; it proposes,
  and you approve in your own words.
- **A runner** that takes the ready work, each task in its own git worktree, until only
  work for you is left.

## Install

You need Go 1.24, git, Node 22 (for the Obsidian plugin), and Claude Code. `gh` is needed to
land pull requests.

```
make install                      # the binary, in ~/.cartographer/bin/carto
make plugin                       # the Obsidian plugin, in obsidian/dist
claude plugin marketplace add .   # the agent plugin
claude plugin install cartographer@cartographer
```

Copy `obsidian/dist/main.js`, `manifest.json`, and `styles.css` into
`<vault>/.obsidian/plugins/cartographer/`, and turn the plugin on.

The vault must be a git repository: every write is a commit, and Undo uses them.

## Start

In a Claude Code session in the vault, ask: "Map the repositories in ~/code/myshop". The
agent proposes the project and the services; you approve. Then tell this machine where the
clones are:

```
carto setup --project <vault>/<Project> --root ~/code
```

Open the project document. Everything else starts there.

## Commands

`carto help` lists every command. The ones you use most:

| Command | Does |
|---|---|
| `carto status` | the dashboard, as text |
| `carto run` | starts agents for the ready work (the **Run now** button runs it) |
| `carto undo` | takes back your last act, or the last approved change |
| `carto check` | validation and lint; exit 1 on a validation error |
| `carto snapshot` | commits edits of documents made outside an act (the plugin and the Stop hook run it) |

## Develop

```
make test    # the core: go vet and go test, with the golden test against the prototype
make e2e     # the Obsidian plugin in a real, separate Obsidian (macOS)
```

The design is in the specification (see `CLAUDE.md`). How the code carries it out is in
`docs/architecture.md`.
