# Cartographer

A schema is the backbone of an application, but it rarely lives in one place. The
same object shows up as a SQL table, a C++ struct, a `.proto` message, a JSON
payload on the wire, a Pydantic model in another service — and as a system grows,
those representations drift apart. There is no single record of what an object
*actually is* in each place, so reconciling the differences is slow, manual, and
easy to get wrong.

Cartographer maps that landscape. You register each object once and attach the
representations that genuinely exist for it in the code. The result is a local,
authoritative picture of how every object is expressed across languages and
services — one you can query, compare across projects, export as a contract, and
view on a canvas.

It is deliberately a documentation tool, not a code generator: it records the
representations that exist, and never invents ones that don't. Discovery is meant
to be driven by an LLM through the built-in MCP server — point Claude Code at a
repository and have it map every schema object it finds.

Cartographer is local-first and in active development. Today it ships the schema
store, the MCP server, and a read-only web canvas.

## How it fits together

- A **store** on local disk holds the schema — one folder per project, one folder
  per object, human-readable YAML plus the raw source snippets. Its location is
  `CARTOGRAPHER_STORE_DIR` (default `~/.cartographer/store`).
- The **MCP server** (`mcp/`) is how schema gets into the store. Claude Code calls
  its tools to register objects and their representations while scanning a repo.
- The **web canvas** (`web/`) reads the same store and draws it. It is read-only on
  schema; the code stays the source of truth.

Both binaries share one store, so anything the MCP server records shows up in the
canvas.

## Prerequisites

- Go 1.25+
- Node.js 20+ and npm (only for building the web canvas)
- Claude Code (for the MCP integration)

## Quickstart

### 1. Build the MCP server and add it to Claude Code

Build the stdio binary:

```
go build -o mcp/server ./mcp
```

Register it with Claude Code (run from the repo root; use an absolute path to the
binary):

```
claude mcp add cartographer \
  -e CARTOGRAPHER_STORE_DIR=$HOME/.cartographer/store \
  -- "$(pwd)/mcp/server"
```

Or add it by hand to your Claude Code MCP config (`.mcp.json`):

```json
{
  "mcpServers": {
    "cartographer": {
      "command": "/absolute/path/to/cartographer/mcp/server",
      "env": { "CARTOGRAPHER_STORE_DIR": "/absolute/path/to/store" }
    }
  }
}
```

Then, in a Claude Code session inside any repository:

```
Use the cartographer MCP server. Scan this repo for schema objects — SQL tables,
proto messages, structs, JSON payloads, Pydantic models — and register each one
under the project "my-service".
```

Claude Code will call the tools below to build up the map.

### 2. Run the web canvas

Build the single-page app (it gets embedded into the server binary), then build
and run the server:

```
cd web/ui && npm install && npm run build && cd ../..
go build -o web/cartographer-web ./web
CARTOGRAPHER_STORE_DIR=$HOME/.cartographer/store ./web/cartographer-web --addr :8099
```

Open <http://localhost:8099>. Point it at the same store the MCP server writes to,
and the canvas fills in as objects are registered.

For UI development with hot reload, run the Vite dev server alongside the Go
server (it proxies `/api` to `:8099`):

```
cd web/ui && npm run dev      # Vite on :5173
go run ./web                  # API on :8099
```

More web build notes: [`web/README.md`](web/README.md).

## MCP tools

At this phase the server exposes eight tools. Each object is an **entity** in a
**project**; an entity has one **aspect** per representation **kind** (`sql`,
`json`, `protobuf`, `cpp_struct`, `pydantic`, …), and each aspect carries the raw
source plus a normalized field list.

| Tool | What it does |
|---|---|
| `register_entity` | Create or update an object in a project (creating the project on first use). Call this before adding aspects. |
| `set_aspect` | Add or replace one representation of an object — its raw source plus normalized fields (name, type as written, nullable, tags like `primary_key`/`foreign_key`, and references for foreign keys). Re-registering a kind overwrites it. |
| `remove_aspect` | Drop one representation from an object, so a re-scan yields an accurate map. |
| `remove_entity` | Remove an object and all its representations from a project. |
| `list_entities` | List objects and their representation kinds. Optionally filter to one project and/or to objects that have a given kind (e.g. only those with a protobuf contract). |
| `get_entity` | Return one object's full detail: every aspect with its kind, source, raw text, and fields. |
| `diff_entities` | Compare two objects. Reports which representation kinds each side has (presence gaps) and the field-level differences within each shared kind — including across two projects, to align the same object between environments. |
| `export_contract` | Export the authoritative schema as YAML for one object or a whole project — the normalized shape, not a copy of raw source. |

## Repository layout

- `internal/model/` — the domain types (project, entity, aspect, field)
- `internal/store/` — file-backed store and in-memory index
- `internal/diff/` — presence-level and field-level comparison
- `internal/contract/` — YAML contract export
- `internal/mcptools/` — MCP tool handlers
- `mcp/` — the stdio MCP server binary
- `internal/layout/` — canvas card positions (presentation state, kept separate from schema)
- `web/` — the Go web server; `web/ui/` is the Vue 3 + TypeScript canvas
