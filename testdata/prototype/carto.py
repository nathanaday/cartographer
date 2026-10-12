#!/usr/bin/env python3
"""Prototype of the Cartographer core (Specification, Draft 4.1).

Reads a project folder and computes obligations, status, staleness, findings,
lint, goal progress, the run queue, and the dashboard. Then it writes the tool
fields, the "What the plugin shows" callouts, and the canvases.
`simulation.yaml` stands in for git, the forge, the claims file, and the clock.
Reachability is approximated: a commit is on the default branch when it has no
ref, or when the Log of its goal records that its repository landed.

    python3 carto.py ../MyShop            # render everything
    python3 carto.py ../MyShop --check    # validation and lint only
"""
import argparse, datetime as dt, glob, hashlib, json, os, re, sys
import yaml

HERE = os.path.dirname(os.path.abspath(__file__))
LINK = re.compile(r"\[\[([^\]|#]*)(?:#([^\]|]*))?(?:\|[^\]]*)?\]\]")
FIELD = re.compile(r"\[(\w+)::\s*((?:\[\[[^\]]*\]\]|[^\]])*)\]")
FM = re.compile(r"\A---\n(.*?)\n---\n", re.S)
CONCLUSIVE = ("met", "met-none", "violated")


# ---------- text helpers ----------
def links(value):
    if not value:
        return []
    if isinstance(value, list):
        return [l for v in value for l in links(v)]
    return [(m.group(1), (m.group(2) or "").lstrip("^")) for m in LINK.finditer(str(value))]


def names(value):
    return [n for n, _ in links(value)]


def fields(text):
    return {k: v.strip() for k, v in FIELD.findall(text)}


def strip_fields(text):
    return re.sub(r"\s*\^\w+\s*$", "", FIELD.sub("", text)).strip()


def when(value):
    if not value:
        return None
    if isinstance(value, dt.datetime):
        return value
    if isinstance(value, dt.date):
        return dt.datetime(value.year, value.month, value.day)
    return dt.datetime.fromisoformat(str(value))


def words(text, n=8):
    w = re.sub(r"\[\[([^\]|#]*)[^\]]*\]\]", r"\1", text).replace("`", "").split()
    return " ".join(w[:n]) + (" …" if len(w) > n else "")


def section(body, title):
    m = re.search(rf"^## {re.escape(title)}\n(.*?)(?=^## |\Z)", body, re.S | re.M)
    return m.group(1) if m else ""


def table_rows(text):
    for line in text.splitlines():
        if line.startswith("|") and not line.startswith("|---"):
            yield [c.strip() for c in line.strip().strip("|").split(" | ")]


def repo_path(location):
    """github.com/org/repo/some/path -> ('repo', 'some/path')."""
    parts = str(location).split("/")
    return parts[2], "/".join(parts[3:])


# ---------- model ----------
class Doc:
    def __init__(self, path):
        self.path = path
        self.name = os.path.splitext(os.path.basename(path))[0]
        text = open(path).read()
        m = FM.match(text)
        self.fm = yaml.safe_load(m.group(1)) if m else {}
        self.body = text[m.end():] if m else text
        self.type = (self.fm.get("type") or "").replace("carto-", "")


class Clause:
    def __init__(self, owner, cid, statement, f, rev=None, pending_goal=None):
        self.owner, self.id, self.statement, self.f = owner, cid, statement, f
        self.state = "pending" if pending_goal else f.get("state", "active")
        self.rev = int(rev or f.get("rev", 1))
        self.scope = f.get("scope", "each")
        self.verify = f.get("verify", "agent")
        self.goal = pending_goal or (names(f.get("goal", "")) or [None])[0]

    @property
    def link(self):
        return f"[[{self.owner}#{self.id}]]"


class Result:
    def __init__(self, check, date, ref, rev, applies, result, fails, evidence, deps, repo=""):
        self.check, self.date, self.ref, self.rev, self.repo = check, date, ref, rev, repo
        self.applies, self.result, self.fails, self.evidence, self.deps = applies, result, fails, evidence, deps


class Obligation:
    def __init__(self, entity, clause, source, goal=None):
        self.entity, self.clause, self.source = entity, clause, source
        self.goal = goal or (clause.goal if clause.state == "pending" else None)
        self.status, self.result, self.stale, self.waived_from = "unverified", None, None, None

    @property
    def pending(self):
        return self.goal is not None


class Project:
    def __init__(self, folder):
        self.folder = os.path.abspath(folder)
        self.sim = yaml.safe_load(open(os.path.join(HERE, "simulation.yaml")))
        self.now = when(self.sim["now"])
        self.docs, self.validation = {}, []
        for p in sorted(glob.glob(os.path.join(self.folder, "**/*.md"), recursive=True)):
            try:
                d = Doc(p)
            except yaml.YAMLError as e:
                self.validation.append(("V1", os.path.basename(p), f"frontmatter is not valid YAML: {e}"))
                continue
            self.docs[d.name] = d
        self.project = self.of("project")[0]
        pf = self.docs[self.project].fm
        self.default = pf.get("default_branch", "main")
        self.stale_after = int(pf.get("stale_after", 30))
        self.nudge_after = int(pf.get("nudge_after", 14))
        self.parse_clauses()
        self.parse_goals()
        self.parse_checks()
        self.parse_waivers()
        self.compute_obligations()
        self.compute_status()
        self.compute_findings()
        self.compute_lint()
        self.validate()

    # ---------- index ----------
    def of(self, t):
        return [n for n, d in self.docs.items() if d.type == t]

    def fm(self, name, key, default=None):
        return self.docs[name].fm.get(key, default) if name in self.docs else default

    def parse_clauses(self):
        self.clauses = []
        for n, d in self.docs.items():
            if d.type not in ("contract", "system", "service", "module", "goal"):
                continue
            body = section(d.body, "Clauses") + section(d.body, "Requirements")
            for m in re.finditer(r"^### (\S+)\n(.*?)(?=^### |^## |\Z)", body, re.S | re.M):
                lines = [l for l in m.group(2).strip().splitlines() if l.strip()]
                stmt = " ".join(l for l in lines if not l.startswith(("[", ">")))
                f = fields(" ".join(l for l in lines if l.startswith("[")))
                c = Clause(n, m.group(1), stmt, f)
                if d.type == "goal":
                    c.state, c.goal = "requirement", n
                self.clauses.append(c)
                if "pending_rev" in f:
                    pend = " ".join(re.sub(r"^> \[!pending\]\s*", "", l).lstrip("> ") for l in lines if l.startswith(">"))
                    self.clauses.append(Clause(n, m.group(1), pend, f, rev=f["pending_rev"], pending_goal=names(f["pending_goal"])[0]))

    def clause(self, owner, cid, pending=False):
        for c in self.clauses:
            if c.owner == owner and c.id == cid and (c.state == "pending") == pending:
                return c
        return None

    def parse_lines(self, name, title):
        out = []
        for line in section(self.docs[name].body, title).splitlines():
            m = re.match(r"^- \[(.)\] (T\d+) \[\[([^\]]+)\]\] (.*)$", line)
            if m:
                rest = m.group(4)
                blk = re.search(r"\^(\w+)\s*$", rest)
                out.append({"box": m.group(1), "id": m.group(2), "target": m.group(3), "text": strip_fields(rest),
                            "f": fields(rest), "block": blk.group(1) if blk else "", "goal": name})
        return out

    def parse_goals(self):
        self.tasks, self.changes = {}, {}
        for g in self.of("goal"):
            self.tasks[g] = self.parse_lines(g, "Tasks")
            acts = []
            for line in section(self.docs[g].body, "Map changes").splitlines():
                m = re.match(r"^- \*\*(add|revise|retire|adopt|attach|use|join|create)\*\* (.*)$", line)
                if m:
                    acts.append((m.group(1), links(m.group(2))))
            self.changes[g] = acts

    def goal_clause_keys(self, g):
        keys = [ls[0] for act, ls in self.changes[g] if act in ("add", "revise", "retire") and ls]
        for f in names(self.fm(g, "resolves")):
            if self.fm(f, "kind") == "violation":
                keys += links(self.fm(f, "clause"))
        return keys

    def goal_status(self, g):
        return self.fm(g, "status")

    def live(self, g):
        return self.goal_status(g) in ("active", "parked")

    def parse_checks(self):
        self.results, self.checks, self.dependencies = {}, {}, {}
        for n in self.of("check"):
            d = self.docs[n]
            if d.fm.get("sweep"):
                for m in re.finditer(r"^## (.+?)\n(.*?)(?=^## |\Z)", d.body, re.S | re.M):
                    title, body = m.group(1).strip(), m.group(2)
                    if title == "Coverage":
                        for cells in table_rows(body):
                            if len(cells) == 4 and cells[0] != "Repository":
                                self.dependencies.setdefault(self.project, []).append(
                                    (when(d.fm["created"]), n, f"{cells[0]}:{cells[1]}", cells[2], cells[3]))
                    elif title in self.docs:
                        f = fields(body.split("###")[0])
                        self.record(f"{n}#{title}", title, when(f["at"]), f.get("ref", self.default),
                                    repo_path(f["repository"])[0] if "repository" in f else "", body)
            else:
                repo = repo_path(d.fm["repository"])[0] if d.fm.get("repository") else ""
                self.record(n, names(d.fm.get("target"))[0], when(d.fm.get("created")), d.fm.get("ref", self.default), repo, d.body)
        for v in self.results.values():
            v.sort(key=lambda r: r.date)

    def record(self, name, target, date, ref, repo, body):
        self.checks[name] = (target, date, ref)
        sub = lambda title: (re.search(rf"^#{{2,3}} {title}\n(.*?)(?=^#{{2,3}} |\Z)", body, re.S | re.M) or [None, ""])[1]
        for cells in table_rows(sub("Obligations")):
            if len(cells) != 7 or not cells[0].startswith("[["):
                continue
            (doc, cid), = links(cells[0])
            deps = [x.strip() for x in re.split(r",\s+(?![^\[]*\]\])", cells[6]) if x.strip()]
            fails = [] if cells[4] in ("—", "") else [x.strip() for x in cells[4].split(",")]
            self.results.setdefault((target, doc, cid), []).append(
                Result(name, date, ref, int(cells[1]), cells[2], cells[3], fails, cells[5], deps, repo))
        for cells in table_rows(sub("Dependencies")):
            if len(cells) == 3 and cells[0] != "Dependency":
                self.dependencies.setdefault(target, []).append((date, name, cells[0], cells[1], cells[2]))

    def landings(self, g):
        """(time, repository, commit, task) for each `landed` line of a goal's Log."""
        out = []
        for m in re.finditer(r"^- (\d{4}-\d\d-\d\d \d\d:\d\d) .*?landed (?:(T\d+) )?([\w-]+) (\w+)", section(self.docs[g].body, "Log"), re.M):
            out.append((dt.datetime.strptime(m.group(1), "%Y-%m-%d %H:%M"), m.group(3), m.group(4), m.group(2)))
        return out

    def task_branch(self, t):
        return f"upkeep/{t['id'].lower()}" if self.fm(t["goal"], "standing") else self.fm(t["goal"], "branch")

    def landed(self, repo, ref):
        for g in self.of("goal"):
            for _, r, _, task in self.landings(g):
                if r == repo and (self.fm(g, "branch") == ref or (task and f"upkeep/{task.lower()}" == ref)):
                    return True
        return False

    def counts(self, r):
        return r.ref == self.default or self.landed(r.repo, r.ref)

    def parse_waivers(self):
        self.waivers = []
        for n, d in self.docs.items():
            for line in section(d.body, "Waivers").splitlines():
                m = re.match(r"^- ((?:lint L\d+ )?\[\[[^\]]+\]\]|dependency `[^`]+`) · (.*)$", line)
                if not m:
                    continue
                tgt = m.group(1)
                kind = "lint" if tgt.startswith("lint") else "dependency" if tgt.startswith("dependency") else "clause"
                key = links(tgt)[0] if kind == "clause" else tgt
                self.waivers.append({"entity": n, "kind": kind, "target": key, "f": fields(m.group(2)), "reason": strip_fields(m.group(2))})

    def waiver_for(self, entity, target, rev=None, fails=None):
        for w in self.waivers:
            if w["entity"] != entity or w["target"] != target:
                continue
            f = w["f"]
            if rev and f.get("rev") and int(f["rev"]) != rev:
                continue
            goal = names(f.get("goal", ""))
            if goal and self.goal_status(goal[0]) == "done":
                continue
            until = f.get("until")
            if until and when(until) < self.now:
                continue
            parts = [p.strip() for p in f.get("parts", "").split(",") if p.strip()]
            if parts and fails and not set(fails) <= set(parts):
                continue
            return w
        return None

    # ---------- membership ----------
    def members(self, system):
        return [s for s in self.of("service") if system in names(self.fm(s, "systems"))]

    def inside(self, entity):
        t = self.docs[entity].type
        if t == "project":
            return [entity] + self.of("system") + self.of("service") + self.of("module")
        if t == "system":
            return [entity] + self.members(entity)
        return [entity]

    def adopts(self, entity):
        return names(self.fm(entity, "adopts"))

    def effective_adopts(self, service):
        out = set(self.adopts(service)) | set(self.adopts(self.project))
        for s in names(self.fm(service, "systems")):
            out |= set(self.adopts(s))
        return out

    def checked_ever(self, entity):
        return any(t == entity for t, _, _ in self.checks.values())

    # ---------- obligations ----------
    def falls_on(self, clause, adopter):
        t = self.docs[adopter].type
        if t == "project":
            return self.of("service") + self.of("module") if clause.scope == "each" else [adopter]
        if t == "system":
            return self.members(adopter) if clause.scope == "each" else [adopter]
        return [adopter]

    def goal_added_adoption(self, contract, adopter):
        for g in self.of("goal"):
            if self.live(g) and ("adopt", [(contract, ""), (adopter, "")]) in self.changes[g]:
                return g
        return None

    def compute_obligations(self):
        self.obligations, self.draft_obligations = [], {}
        seen = set()

        def add(o):
            k = (o.entity, o.clause.owner, o.clause.id, o.clause.rev)
            if k not in seen:
                seen.add(k)
                self.obligations.append(o)
        for a in [self.project] + self.of("system") + self.of("service") + self.of("module"):
            for c in self.adopts(a):
                via = self.goal_added_adoption(c, a)
                for cl in self.clauses:
                    if cl.owner == c and cl.state in ("active", "pending"):
                        if cl.state == "pending" and not self.live(cl.goal):
                            continue
                        for e in self.falls_on(cl, a):
                            add(Obligation(e, cl, a, goal=via))
            for cl in self.clauses:
                if cl.owner == a and cl.state == "active":
                    add(Obligation(a, cl, "local"))
        for cl in self.clauses:
            if cl.state == "requirement":
                on = names(cl.f.get("on", ""))
                if not on:
                    continue
                o = Obligation(on[0], cl, cl.owner, goal=cl.owner)
                if self.live(cl.owner):
                    add(o)
                elif self.goal_status(cl.owner) == "draft":
                    self.draft_obligations.setdefault(cl.owner, []).append(o)

    def active_of(self, entity):
        return [o for o in self.obligations if o.entity == entity and not o.pending]

    def pending_of(self, entity):
        return [o for o in self.obligations if o.entity == entity and o.pending]

    # ---------- status ----------
    def implicit_deps(self, o):
        out = []
        owner = self.docs[o.clause.owner]
        if owner.type == "contract" and owner.fm.get("definition"):
            r, p = repo_path(owner.fm["definition"])
            out.append(f"{r}:{p}")
        if self.docs[o.entity].type == "system":
            for s in self.members(o.entity):
                r, _ = repo_path(self.fm(s, "repository"))
                out.append(f"{r}:{str(self.fm(s, 'path', '')).lstrip('./')}")
        return out

    def stale_commits(self, r, deps, depth=0):
        """Commits on the default branch after the result that touch a path dependency, and stale citations."""
        shas, external = set(), False
        for dep in deps:
            if dep.startswith("external:"):
                external = external or (self.now - r.date).days > self.stale_after
            elif dep.startswith("[[") and depth < 3:
                (doc, anchor), = links(dep)
                cited = [x for (e, d, c), rs in self.results.items() for x in rs if x.check == f"{doc}#{anchor}" or x.check == doc]
                for x in cited:
                    n, _ = self.stale_commits(x, x.deps, depth + 1)
                    if n:
                        shas.add(f"cited:{x.check}")
            elif ":" in dep:
                repo, path = dep.split(":", 1)
                for c in self.sim["commits"].get(repo, []) or []:
                    on_default = c.get("ref", self.default) == self.default or self.landed(repo, c["ref"])
                    if on_default and when(c["date"]) > r.date and any(p.startswith(path.rstrip("/")) for p in c["paths"]):
                        shas.add(c["sha"])
        return len(shas), external

    def latest(self, o, at=None, any_ref=False):
        rs = [r for r in self.results.get((o.entity, o.clause.owner, o.clause.id), [])
              if r.rev == o.clause.rev and r.result in CONCLUSIVE and (any_ref or self.counts(r))
              and (at is None or r.date <= at)]
        return rs[-1] if rs else None

    def status_at(self, o, at=None):
        r = self.latest(o, at, any_ref=o.pending)
        if not r:
            return "unverified", None
        return r.result, r

    def compute_status(self):
        for o in self.obligations + [o for v in self.draft_obligations.values() for o in v]:
            o.status, o.result = self.status_at(o)
            if o.result and not o.pending:
                n, external = self.stale_commits(o.result, o.result.deps + self.implicit_deps(o))
                age = (self.now - o.result.date).days
                timed = o.clause.scope == "whole" or o.clause.verify in ("metric", "review")
                if n or external or (timed and age > self.stale_after):
                    o.stale = f"{n} commits, {age} days" if n else f"{age} days"
            if o.result and o.pending and o.result.ref != self.default:
                for c in self.sim["commits"].get(o.result.repo, []) or []:
                    if c.get("ref") == o.result.ref and when(c["date"]) > o.result.date and \
                            any(any(p.startswith(d.split(":", 1)[1].rstrip("/")) for p in c["paths"]) for d in o.result.deps if ":" in d and not d.startswith(("[[", "external:"))):
                        o.stale = "branch moved"
            if not o.pending and o.status == "violated" and self.waiver_for(o.entity, (o.clause.owner, o.clause.id), o.clause.rev, o.result.fails):
                o.waived_from, o.status = o.status, "waived"

    def figures(self, obs):
        return {"met": sum(o.status in ("met", "met-none") for o in obs), "waived": sum(o.status == "waived" for o in obs),
                "violated": sum(o.status == "violated" for o in obs), "unverified": sum(o.status == "unverified" for o in obs),
                "stale": sum(bool(o.stale) for o in obs), "total": len(obs)}

    def compliance(self, e):
        return self.figures(self.active_of(e))

    def rollup(self, e):
        return self.figures([o for x in self.inside(e) for o in self.active_of(x)])

    # ---------- findings ----------
    def compute_findings(self):
        self.finding = {}
        for n in self.of("finding"):
            kind = self.fm(n, "kind")
            ent = (names(self.fm(n, "entity")) or [""])[0]
            st, contract, resolved, resolved_at = "open", "", "", None
            if kind == "violation":
                (doc, cid), = links(self.fm(n, "clause"))
                contract = doc if self.docs[doc].type == "contract" else ""
                o = next((o for o in self.obligations if o.entity == ent and o.clause.owner == doc and o.clause.id == cid and not o.pending), None)
                if not o:
                    st, resolved = "resolved", "obligation ended"
                elif o.status == "waived":
                    st = "waived"
                elif o.status in ("met", "met-none"):
                    st, resolved, resolved_at = "resolved", o.result.date.date().isoformat(), o.result.date
            else:
                key = str(self.fm(n, "key", ""))
                target = ent or self.project
                about = names(self.fm(n, "about"))
                opts = fields(section(self.docs[n].body, "Options"))
                rows = sorted(r for r in self.dependencies.get(target, []) if key.split(":")[-1] in r[2].replace("`", ""))
                if kind == "divergence" and about and about[0] in names(self.fm(ent, "interfaces")):
                    st, resolved = "resolved", str(opts.get("on", ""))
                    resolved_at = when(resolved) if resolved[:4].isdigit() else None
                elif rows and rows[-1][3] in ("convergence", "mapped"):
                    st, resolved, resolved_at = "resolved", rows[-1][0].date().isoformat(), rows[-1][0]
                if self.waiver_for(target, f"dependency `{key}`"):
                    st = "waived"
            chosen = fields(section(self.docs[n].body, "Options")).get("chosen")
            res = [f"[[{t['goal']}#^{t['block']}]]" for g in self.tasks for t in self.tasks[g] if n in names(t["f"].get("finding", ""))]
            res += [f"[[{g}]]" for g in self.of("goal") if n in names(self.fm(g, "resolves"))]
            self.finding[n] = {"status": st, "contract": f"[[{contract}]]" if contract else "", "resolved": resolved, "resolved_at": resolved_at,
                               "resolution": res, "chosen": chosen, "entity": ent, "kind": kind}

    # ---------- goals ----------
    def goal_obligations(self, g):
        keys = set(self.goal_clause_keys(g))
        obs = [o for o in self.obligations if o.goal == g or (o.clause.owner, o.clause.id) in keys]
        return sorted(obs + self.draft_obligations.get(g, []), key=lambda o: (o.clause.owner != g, o.clause.id, o.entity))

    def goal_met(self, o, at=None):
        st = self.status_at(o, at)[0] if (o.pending or at is not None) else o.status
        return st in ("met", "met-none")

    def goal_progress(self, g, at=None):
        obs = self.goal_obligations(g)
        return sum(self.goal_met(o, at) for o in obs), len(obs)

    def goal_tasks(self, g, at=None):
        ts = self.tasks[g]
        def closed(t):
            return t["box"] in "x-" and (at is None or (when(t["f"].get("done")) or self.now) <= at)
        return sum(closed(t) for t in ts), len(ts)

    def goal_branches(self, g):
        """Repository -> {'state': 'landed' | 'open', 'at': time} for a goal's branch."""
        if self.fm(g, "standing"):
            return {}
        repos = {self.task_repo(t) for t in self.tasks[g] if t["box"] != "-" and self.task_repo(t)}
        landed = {r: when_ for when_, r, _, _ in self.landings(g)}
        return {r: {"state": "landed", "at": landed[r]} if r in landed else {"state": "open"} for r in sorted(repos)}

    def blocked(self, t):
        return [x for x in names(t["f"].get("blocked", "")) if x in self.docs
                and (self.fm(x, "status") == "open" or self.finding.get(x, {}).get("status") == "open")]

    def claims(self, g=None):
        return [c for c in self.sim.get("claims", []) or [] if g is None or c["goal"] == g]

    def task_repo(self, t):
        loc = self.fm(t["target"], "repository") or self.fm(t["target"], "definition") or ""
        return repo_path(loc)[0] if str(loc).count("/") >= 2 else t["f"].get("repo", "")

    def ready(self, g):
        if not self.live(g) or (self.fm(g, "defer") and when(self.fm(g, "defer")) > self.now):
            return []
        claimed = {c["task"] for c in self.claims(g)}
        done = {t["id"] for t in self.tasks[g] if t["box"] in "x-"}
        busy = {(c["repo"], self.fm(c["goal"], "branch")) for c in self.claims()}
        out = []
        for t in self.tasks[g]:
            after = [a.strip() for a in t["f"].get("after", "").split(",") if a.strip()]
            if t["box"] == " " and t["id"] not in claimed and not self.blocked(t) and t["f"].get("user") != "true" \
                    and int(t["f"].get("attempts", 0) or 0) < 2 \
                    and all(a in done for a in after) and (self.task_repo(t), self.task_branch(t)) not in busy:
                out.append(t)
        return out

    def questions_blocking(self, g):
        return sorted({x for t in self.tasks[g] for x in names(t["f"].get("blocked", ""))
                       if self.docs.get(x) and self.docs[x].type == "question" and self.fm(x, "status") == "open"})

    def affects(self, g):
        return sorted({o.entity for o in self.goal_obligations(g)})

    # ---------- lint and validation ----------
    def compute_lint(self):
        out = []
        for s in self.of("service"):
            eff = self.effective_adopts(s)
            for i in names(self.fm(s, "interfaces")):
                for sc in names(self.fm(i, "schemas")):
                    if sc not in eff and not any(w["entity"] == s and w["kind"] == "lint" and w["target"].startswith("lint L1") for w in self.waivers):
                        out.append(("L1", s, f"[[{s}]] is attached to [[{i}]], whose schema is [[{sc}]], but does not adopt it"))
            if not names(self.fm(s, "systems")):
                out.append(("L3", s, f"[[{s}]] belongs to no system"))
        for i in self.of("interface"):
            att = [s for s in self.of("service") if i in names(self.fm(s, "interfaces"))]
            if len(att) < 2 and not self.fm(i, "external"):
                out.append(("L2", i, f"[[{i}]] has {len(att)} attached service and no external party"))
        self.lint = out

    def validate(self):
        codes = {}
        for n, d in self.docs.items():
            if d.fm.get("code"):
                codes.setdefault(d.fm["code"], []).append(n)
        self.validation += [("V2", v[0], f"code {k} is used by {', '.join(v)}") for k, v in codes.items() if len(v) > 1]
        for c in self.clauses:
            if c.state == "requirement" and not names(c.f.get("on", "")):
                self.validation.append(("V4", c.owner, f"{c.link} has no on"))
        for (e, doc, cid), rs in self.results.items():
            cl = self.clause(doc, cid)
            for r in rs:
                if r.result == "met-none" and not r.applies.startswith("none:"):
                    self.validation.append(("V5", e, f"met-none for {cid} in {r.check} has no reason"))
                if r.result == "inconclusive" and "reason:" not in r.evidence:
                    self.validation.append(("V5", e, f"inconclusive for {cid} in {r.check} has no reason"))
                if not r.deps and r.result in CONCLUSIVE:
                    self.validation.append(("V5", e, f"{cid} in {r.check} has no dependency"))
        owner = {}
        for g in self.of("goal"):
            if self.goal_status(g) in ("done", "dropped"):
                continue
            for act, ls in self.changes[g]:
                if act in ("add", "revise", "retire") and ls:
                    if ls[0] in owner:
                        self.validation.append(("V6", g, f"{g} and {owner[ls[0]]} both change {ls[0][0]}#{ls[0][1]}"))
                    owner[ls[0]] = g
        for g in self.of("goal"):
            if self.fm(g, "standing"):
                for t in self.tasks[g]:
                    if not any(k in t["f"] for k in ("finding", "question", "by")):
                        self.validation.append(("V7", g, f"upkeep task {t['id']} names no source"))
        for o in self.obligations:
            if not o.pending and o.status == "violated":
                if not any(f["kind"] == "violation" and f["entity"] == o.entity and f["status"] == "open" for f in self.finding.values()):
                    self.validation.append(("core", o.entity, f"{o.clause.link} is violated and has no open finding"))

    # ---------- colors ----------
    def color(self, e):
        t = self.docs[e].type
        if any(f["kind"] == "violation" and f["status"] == "open" and f["entity"] in self.inside(e) for f in self.finding.values()):
            return "1"
        if t != "project" and self.active_of(e) and not self.checked_ever(e):
            return None
        scope = [x for x in (self.inside(e) if t in ("system", "project") else [e]) if self.checked_ever(x)]
        f = self.figures([o for x in scope for o in self.active_of(x)])
        if not f["total"]:
            return None
        return "3" if f["unverified"] else "5" if f["waived"] else "4"

    def contract_red(self, contract, owner):
        return any(f["kind"] == "violation" and f["status"] == "open" and f["contract"] == f"[[{contract}]]" and f["entity"] in self.inside(owner)
                   for f in self.finding.values())


# ---------- rendering ----------
def L(n):
    return f"[[{n}]]"


def status_text(o):
    s = {"met-none": "met (none)", "violated": "**violated**"}.get(o.status, o.status)
    if o.status == "waived":
        s = f"waived ({o.waived_from})"
    return s + (f" · stale {o.stale}" if o.stale else "")


def fig_text(f):
    return f"met {f['met']}" + (f" · waived {f['waived']}" if f["waived"] else "") + f" / {f['total']}"


def table(head, rows):
    if not rows:
        return []
    return ["| " + " | ".join(head) + " |", "|" + "---|" * len(head)] + ["| " + " | ".join(str(c) for c in r) + " |" for r in rows]


def source_text(P, o):
    if o.source == "local":
        return "local"
    if o.source == o.entity:
        return "adopts"
    return L(o.source) + (" (whole)" if o.clause.scope == "whole" and P.docs[o.source].type != "goal" else "")


def result_cell(P, o):
    if o.result:
        return L(o.result.check)
    rs = P.results.get((o.entity, o.clause.owner, o.clause.id), [])
    return f"inconclusive in {L(rs[-1].check)}" if rs else ("none" if P.checked_ever(o.entity) else "not yet checked")


def r_obligations(P, e):
    obs = sorted(P.active_of(e), key=lambda o: (o.clause.owner, o.clause.id))
    out = table(["Clause", "Rule", "From", "Status", "Result"],
                [(o.clause.link, words(o.clause.statement), source_text(P, o), status_text(o), result_cell(P, o)) for o in obs]) or ["No active obligations."]
    if obs:
        out += ["", f"Compliance: {fig_text(P.figures(obs))}." + ("" if P.checked_ever(e) else " Not yet checked: in the sweep queue.")]
    pend = sorted(P.pending_of(e), key=lambda o: o.clause.id)
    if pend:
        rows = []
        for o in pend:
            st = o.status if o.result else "unverified"
            if o.result and o.result.ref != P.default:
                st += f" on `{o.result.ref}`"
            rows.append((o.clause.link, L(o.goal), st))
        out += ["", "Pending, for goals:", ""] + table(["Clause", "Goal", "Result"], rows)
    fs = [n for n, f in P.finding.items() if f["entity"] == e and f["status"] == "open"]
    if fs:
        out += ["", "Open findings: " + " · ".join(L(f) for f in fs)]
    ls = [m for r, x, m in P.lint if x == e]
    if ls:
        out += ["", "Lint: " + "; ".join(ls) + "."]
    return out


def r_children(P, e):
    t = P.docs[e].type
    if t == "project":
        rows = [(L(s), " · ".join(L(m) for m in P.members(s)), fig_text(P.rollup(s)), P.rollup(s)["violated"]) for s in P.of("system")]
        mods = P.figures([o for m in P.of("module") for o in P.active_of(m)])
        rows.append(("Modules", " · ".join(L(m) for m in P.of("module")), fig_text(mods), mods["violated"]))
        return table(["System", "Services", "Rollup", "Violated"], rows)
    if t == "system":
        return table(["Service", "Compliance", "Violated"], [(L(m), fig_text(P.compliance(m)), P.compliance(m)["violated"]) for m in P.members(e)])
    if t == "module":
        return [" · ".join(L(s) for s in P.of("service") if e in names(P.fm(s, "uses"))) or "No service uses it."]
    return []


def agents_text(P, g):
    return " · ".join(f"{c['task']} in {c['repo']}: [[{c['log']}]]" for c in P.claims(g)) or "none"


def task_state(P, t):
    claim = next((c for c in P.claims(t["goal"]) if c["task"] == t["id"]), None)
    if P.blocked(t):
        return "waits on " + " · ".join(L(b) for b in P.blocked(t))
    if claim:
        return f"an agent works on it in {claim['repo']}: [[{claim['log']}]]"
    if int(t["f"].get("attempts", 0) or 0) >= 2:
        return "stuck"
    return "open"


def r_work(P, e):
    out = []
    for g in P.of("goal"):
        for t in P.tasks[g]:
            if t["target"] == e and t["box"] not in "x-":
                s = task_state(P, t)
                out.append(f"- {t['id']} of {L(g)} ({P.goal_status(g)}): {t['text']}. {s[0].upper() + s[1:]}.")
    for q in P.of("question"):
        if P.fm(q, "status") == "open" and e in names(P.fm(q, "about")):
            out.append(f"- Question for you: {L(q)}.")
    for n, f in P.finding.items():
        if f["entity"] == e and f["status"] == "open":
            out.append(f"- Open finding: {L(n)}.")
    return out or ["No open work."]


def r_adopters(P, c):
    cls = sorted([cl for cl in P.clauses if cl.owner == c and cl.state in ("active", "pending")], key=lambda x: int(re.sub(r"\D", "", x.id) or 0))
    ents = sorted({o.entity for o in P.obligations if o.clause.owner == c})
    short = {"met": "met", "met-none": "met (none)", "violated": "**violated**", "waived": "waived", "unverified": "unverified"}
    rows = []
    for e in ents:
        row = [L(e)]
        for cl in cls:
            o = next((o for o in P.obligations if o.entity == e and o.clause is cl), None)
            cell = "" if not o else short[o.status] + (" · stale" if o.stale else "")
            if o and o.pending and o.result and o.result.ref != P.default:
                cell += " (branch)"
            row.append(cell)
        rows.append(row)
    out = table(["Entity"] + [cl.id + (" (pending)" if cl.state == "pending" else "") for cl in cls], rows) or ["No adopters."]
    adopters = [a for a in [P.project] + P.of("system") + P.of("service") + P.of("module") if c in P.adopts(a)]
    out += ["", "Adopted by " + " · ".join(L(a) for a in adopters) + ". **Adopt for…**"]
    fs = [n for n, f in P.finding.items() if f["contract"] == L(c) and f["status"] == "open"]
    if fs:
        out += ["", "Open findings: " + " · ".join(L(f) for f in fs)]
    ls = [m for r, e, m in P.lint if L(c) in m]
    if ls:
        out += ["", "Lint: " + "; ".join(ls) + "."]
    return out


def r_attached(P, i):
    att = [s for s in P.of("service") if i in names(P.fm(s, "interfaces"))]
    out = ["Attached: " + (" · ".join(L(s) for s in att) or "none")]
    if P.fm(i, "external"):
        out.append(f"External: {P.fm(i, 'external')}")
    ls = [m for r, e, m in P.lint if L(i) in m]
    if ls:
        out += ["", "Lint: " + "; ".join(ls) + "."]
    return out


def branch_text(P, g):
    bs = P.goal_branches(g)
    return f"landed {sum(b['state'] == 'landed' for b in bs.values())}/{len(bs)}" if bs else ""


def progress_line(P, g, waiting=True):
    d, k = P.goal_tasks(g)
    if P.fm(g, "standing"):
        return f"tasks {d}/{k} · open {k - d}"
    m, n = P.goal_progress(g)
    parts = [f"obligations {m}/{n}", f"tasks {d}/{k}"] + ([branch_text(P, g)] if branch_text(P, g) and P.goal_status(g) != "draft" else [])
    if waiting:
        qs = P.questions_blocking(g)
        if qs:
            parts.append(f"waiting on you: {len(qs)} question" + ("s" if len(qs) > 1 else ""))
        if P.goal_status(g) == "draft":
            parts.append("waiting on you: approval" + (f", deferred until {P.fm(g, 'defer')}" if P.fm(g, "defer") else ""))
    return " · ".join(parts)


def r_progress(P, g):
    out = ["**" + progress_line(P, g) + "**", ""]
    if not P.fm(g, "standing"):
        rows = []
        for o in P.goal_obligations(g):
            st = o.status if (o.result and o.status != "waived") else ("waived (does not count)" if o.status == "waived" else "unverified")
            rs = P.results.get((o.entity, o.clause.owner, o.clause.id), [])
            last = rs[-1] if rs else None
            if last and last.result == "inconclusive" and not o.result:
                st = "inconclusive"
            where = f" on `{last.ref}`" if last and last.ref != P.default else ""
            rows.append((o.clause.link, L(o.entity), st + where, L(last.check) if last else "none"))
        out += table(["Clause", "On", "Result", "Last check"], rows)
        bs = P.goal_branches(g)
        if bs:
            out += ["", f"Branch `{P.fm(g, 'branch')}`: " + " · ".join(f"{r} {b['state']}" for r, b in bs.items())]
    qs = P.questions_blocking(g)
    if qs:
        out += ["", "Waiting on you: " + " · ".join(L(q) for q in qs)]
    cl = P.claims(g)
    if cl:
        out += ["", "Agents: " + agents_text(P, g)]
    ready = P.ready(g)
    if ready:
        out += ["", "Ready for the run queue: " + ", ".join(t["id"] for t in ready)]
    return out


def reviewed(P, g):
    """The last of the Log's "ready to land", "not ready to land", and "Tn added" lines is "ready to land"."""
    ms = re.findall(r"^- \d{4}-\d\d-\d\d \d\d:\d\d (ready to land|not ready to land|T\d+ added)", section(P.docs[g].body, "Log"), re.M)
    return bool(ms) and ms[-1] == "ready to land"


def ready_to_land(P, g):
    """Goal: all tasks done and a repository not landed. Upkeep: done tasks whose own branch has not landed."""
    if P.fm(g, "standing"):
        return [t for t in P.tasks[g] if t["box"] == "x" and t["f"].get("commit") and not P.landed(P.task_repo(t), P.task_branch(t))]
    d, k = P.goal_tasks(g)
    return P.live(g) and k and d == k and any(b["state"] != "landed" for b in P.goal_branches(g).values())


def queue(P):
    tasks = [t for g in P.of("goal") for t in P.ready(g)]
    lands = sum(bool(ready_to_land(P, g)) for g in P.of("goal"))
    def waits(o):
        rs = P.results.get((o.entity, o.clause.owner, o.clause.id), [])
        return rs and rs[-1].result == "inconclusive" and any(P.fm(q, "status") == "open" and o.entity in names(P.fm(q, "about"))
                                                               and o.clause.owner in names(P.fm(q, "about")) for q in P.of("question"))
    sweep = sum((o.status == "unverified" and not waits(o)) or bool(o.stale) for o in P.obligations if not o.pending)
    return {"tasks": tasks, "lands": lands, "sweep": sweep}


def r_goals(P, p):
    rows = []
    for g in P.of("goal"):
        if P.goal_status(g) in ("done", "dropped"):
            continue
        wait = [L(q) for q in P.questions_blocking(g)] + (["approval"] if P.goal_status(g) == "draft" else [])
        status = "standing" if P.fm(g, "standing") else P.goal_status(g)
        if P.fm(g, "defer") and when(P.fm(g, "defer")) > P.now:
            status += f", deferred until {P.fm(g, 'defer')}"
        agents = agents_text(P, g)
        full = len(P.claims()) >= int(P.fm(P.project, "max_agents", 3))
        ready = ", ".join(t["id"] for t in P.ready(g))
        rows.append((L(g), status, progress_line(P, g, waiting=False), (ready + (" (no free slot)" if full else "")) if ready else "—",
                     " · ".join(wait) or "—", agents, str(P.fm(g, "updated"))[:16].replace("T", " ")))
    q = queue(P)
    return table(["Goal", "Status", "Progress", "Ready", "Waiting on you", "Agents", "Last activity"], rows) + [
        "", f"Run queue: {len(q['tasks'])} ready task{'s' if len(q['tasks']) != 1 else ''} · {len(P.claims())} agent{'s' if len(P.claims()) != 1 else ''} at work of {P.fm(P.project, 'max_agents', 3)} · "
            f"{q['sweep']} obligations to sweep, next sweep {when(P.sim['next_sweep']):%Y-%m-%d %H:%M}. **Run now**"]


def options(P, doc):
    out = []
    for m in re.finditer(r"^(\d+)\. \*\*(.+?)\*\*(.*)$", section(P.docs[doc].body, "Options"), re.M):
        rec = "recommended" in m.group(2) + m.group(3)
        out.append((m.group(1), re.sub(r"\s*\(recommended\)", "", m.group(2)).rstrip("."), rec))
    return out


def option_line(opts):
    s = " · ".join(f"{n}. {t}" + (" ★" if rec else "") for n, t, rec in opts)
    return f"  {s}" + (" · **Accept recommended**" if any(r for _, _, r in opts) else "") + " · **Not now until…**"


def r_attention(P, p):
    items, deferred = [], []
    def put(item):
        (deferred if item[3] and when(item[3]) > P.now else items).append(item)
    for q in P.of("question"):
        if P.fm(q, "status") != "open":
            continue
        blocks = [(t["goal"], t["id"]) for g in P.tasks for t in P.tasks[g] if q in names(t["f"].get("blocked", ""))]
        tail = " Unblocks " + ", ".join(f"{tid} of {L(g)}" for g, tid in blocks) + "." if blocks else " Blocks nothing."
        put((len(blocks), when(P.fm(q, "created")), [f"- **Answer** {L(q)}.{tail}", option_line(options(P, q))], P.fm(q, "defer")))
    for g in P.of("goal"):
        if P.goal_status(g) == "draft":
            put((len(P.tasks[g]), when(P.fm(g, "created")),
                 [f"- **Approve** the draft goal {L(g)}. Starts {len(P.tasks[g])} tasks.", "  Approve · Drop · **Not now until…**"], P.fm(g, "defer")))
        if P.fm(g, "standing") and P.fm(P.project, "landing") == "user":
            for t in ready_to_land(P, g):
                put((1, when(t["f"].get("done")), [f"- **Land** {t['id']} of {L(g)}: {t['text'].removesuffix('.')}. Done at {t['f']['commit'][:7]}; {P.task_repo(t)}, branch `{P.task_branch(t)}`.",
                                                   "  Land · Request changes · **Not now until…**"], None))
        elif ready_to_land(P, g) and P.fm(P.project, "landing") == "user" and reviewed(P, g):
            m, n = P.goal_progress(g)
            put((len(P.tasks[g]), P.now, [f"- **Land** {L(g)}: reviewer passed; obligations {m}/{n} met on the branch; " +
                 ", ".join(r for r, b in P.goal_branches(g).items() if b["state"] != "landed") + ".", "  Land · Request changes · **Not now until…**"], None))
        for t in P.tasks[g]:
            if t["box"] == " " and int(t["f"].get("attempts", 0) or 0) >= 2:
                put((1, P.now, [f"- **Unstick** {t['id']} of {L(g)}: failed twice.", "  Retry with a hint · I'll do it · Drop · **Not now until…**"], None))
    for n, f in P.finding.items():
        if f["status"] != "open":
            continue
        idle = (P.now - when(P.fm(n, "updated"))).days
        stuck = f["chosen"] and idle >= P.nudge_after
        if not f["chosen"] or stuck:
            why = f" Its chosen option has not moved in {idle} days." if stuck else ""
            put((1, when(P.fm(n, "found")), [f"- **Decide** on {L(n)}.{why}", option_line(options(P, n))], P.fm(n, "defer")))
    held = {str(P.fm(q, "holds")) for q in P.of("question") if P.fm(q, "status") == "open" and P.fm(q, "holds")}
    for rule, e, m in P.lint:
        if f"lint {rule} [[{e}]]" not in held:
            put((0, P.now, [f"- **Decide** on lint {rule}: {m}.", "  1. Fix it on the map ★ · 2. Waive · **Accept recommended** · **Not now until…**"], None))
    for w in P.waivers:
        u = w["f"].get("until")
        if u and 0 <= (when(u) - P.now).days <= 7:
            put((0, when(u), [f"- **Renew or end** the waiver in {L(w['entity'])} that ends {u}."], None))
    for g in P.of("goal"):
        for t in P.tasks[g]:
            due = t["f"].get("due")
            if t["box"] == " " and t["f"].get("user") == "true" and due and (when(due) - P.now).days <= 7:
                put((0, when(due), [f"- **Do** {t['id']} of {L(g)}: {t['text']}, due {due}."], None))
    items.sort(key=lambda x: (-x[0], x[1]))
    out = [l for it in items for l in it[2]] or ["Nothing waits for you."]
    if deferred:
        out += ["", f"Deferred: {len(deferred)} (" + ", ".join(f"until {d[3]}" for d in deferred) + ")."]
    return out


def r_health(P, p):
    out = []
    for n, f in P.finding.items():
        if f["status"] not in ("open", "waived"):
            continue
        res = f["resolution"]
        where = ""
        if res:
            (doc, anchor), = links(res[0])
            task = next((t for t in P.tasks.get(doc, []) if t["block"] == anchor), None)
            if task:
                c = next((c for c in P.claims() if c["task"] == task["id"] and c["goal"] == doc), None)
                if task["box"] == "x":
                    state = "landed" if P.landed(P.task_repo(task), P.task_branch(task)) else "done, waiting for you to land"
                else:
                    b = P.blocked(task)
                    state = "an agent works on it" if c else f"waiting for your answer to {L(b[0])}" if b else "waiting in the run queue"
                where = f" → {doc} {task['id']}, {state}"
            else:
                where = f" → {res[0]} ({P.goal_status(doc)}" + (f", deferred until {P.fm(doc, 'defer')}" if P.fm(doc, "defer") else "") + ")"
        if f["status"] == "waived":
            w = next((w for w in P.waivers if f["entity"] == w["entity"]), None)
            where = f" · waived until {w['f'].get('until')}" + where if w else where
        out.append(f"- {f['kind'].capitalize()}: {L(n)}{where}.")
    for o in P.obligations:
        if not o.pending and o.status == "unverified" and P.results.get((o.entity, o.clause.owner, o.clause.id)):
            q = next((q for q in P.of("question") if P.fm(q, "status") == "open" and o.entity in names(P.fm(q, "about"))
                      and o.clause.owner in names(P.fm(q, "about"))), None)
            out.append(f"- Inconclusive: {o.clause.link} on {L(o.entity)}" + (f" → {L(q)}" if q else "") + ".")
    for rule, e, m in P.lint:
        out.append(f"- Lint {rule}: {m}.")
    never = [e for e in P.of("system") + P.of("service") + P.of("module") if not P.checked_ever(e) and P.active_of(e)]
    if never:
        out.append("- Never checked: " + " · ".join(L(e) for e in never) + ".")
    q = queue(P)
    stale = [o for o in P.obligations if not o.pending and o.stale]
    oldest = max(((P.now - o.result.date).days for o in stale), default=0)
    out.append(f"- Sweep queue: {q['sweep']} obligations ({len(stale)} stale, oldest {oldest} days), next sweep {when(P.sim['next_sweep']):%Y-%m-%d %H:%M}."
               + (" **Not draining.**" if oldest > P.stale_after else ""))
    return out if len(out) > 1 else ["All obligations met."]


def r_since(P, p):
    lv = when(P.sim["last_visit"])
    out = [f"Since {lv:%Y-%m-%d %H:%M}. **Mark seen**", ""]
    for g in P.of("goal"):
        if P.fm(g, "standing"):
            continue
        m0, n0 = P.goal_progress(g, at=lv)
        m1, n1 = P.goal_progress(g)
        d0, k0 = P.goal_tasks(g, at=lv)
        d1, k1 = P.goal_tasks(g)
        if (m0, d0) != (m1, d1):
            out.append(f"- {L(g)}: obligations {m0}/{n0} → {m1}/{n1}, tasks {d0}/{k0} → {d1}/{k1}.")
    for g in P.of("goal"):
        closes = re.findall(r"^- (\d{4}-\d\d-\d\d \d\d:\d\d) every obligation met and every task closed", section(P.docs[g].body, "Log"), re.M)
        if P.fm(g, "status") == "done" and closes and dt.datetime.strptime(closes[-1], "%Y-%m-%d %H:%M") > lv:
            follow = ("\n" + section(P.docs[g].body, "Task details")).count("\n- After the goal lands:")
            out.append(f"- Closed: {L(g)}" + (f", with {follow} follow-up{'s' if follow != 1 else ''} for after the landing in its Task details" if follow else "") + ".")
    for g in P.of("goal"):
        for at, repo, sha, task in P.landings(g):
            if at > lv:
                out.append(f"- Landed: {repo}" + (f" ({task})" if task else "") + f" of {L(g)}, at {sha}.")
    newly = {"met": {}, "violated": {}}
    for o in P.obligations:
        if o.pending:
            continue
        before, now = P.status_at(o, lv)[0], o.status
        key = "met" if now in ("met", "met-none") else now
        if key in newly and before != now:
            newly[key].setdefault(o.entity, []).append(o.clause.id)
    def grouped(d):
        return ", ".join(f"{L(e)} ({len(ids)}{', its first check' if all(P.status_at(o, lv)[0] == 'unverified' for o in P.active_of(e)) and len(ids) > 3 else ': ' + ', '.join(ids)})"
                         for e, ids in d.items()) or "none"
    out.append("- Newly met on the default branch: " + grouped(newly["met"]) + ". Newly violated: " + grouped(newly["violated"]) + ".")
    opened = [n for n in P.finding if when(P.fm(n, "created") or P.fm(n, "found")) > lv]
    resolved = [n for n, f in P.finding.items() if f["status"] == "resolved" and f["resolved_at"] and f["resolved_at"] > lv]
    def findings(ns):
        if len(ns) <= 5:
            return " · ".join(L(f) for f in ns) or "none"
        kinds = [("violation", "violation", "violations"), ("divergence", "undeclared dependency", "undeclared dependencies"),
                 ("absence", "missing relation", "missing relations"), ("unmapped", "unmapped entry point", "unmapped entry points")]
        parts = [f"{k} {one if k == 1 else many}" for kind, one, many in kinds if (k := sum(P.finding[n]["kind"] == kind for n in ns))]
        return f"{len(ns)} ({', '.join(parts)}; Health lists them)"
    out.append("- Findings opened: " + findings(opened) + ". Resolved: " + findings(resolved) + ".")
    answered = [q for q in P.of("question") if P.fm(q, "answered") and when(P.fm(q, "answered")) > lv]
    out.append("- Questions answered: " + (" · ".join(L(q) for q in answered) or "none") + ".")
    return out


RENDER = {"obligations": r_obligations, "children": r_children, "work": r_work, "adopters": r_adopters, "attached": r_attached,
          "progress": r_progress, "goals": r_goals, "health": r_health, "attention": r_attention, "since": r_since}


def render_doc(P, d):
    text = open(d.path).read()
    def sub(m):
        lines = RENDER[m.group(1)](P, d.name)
        return f"```cartographer-{m.group(1)}\n```\n\n> [!example] What the plugin shows\n" + "\n".join("> " + l if l else ">" for l in lines) + "\n"
    return re.sub(r"```cartographer-(\w+)\n```\n(?:\n> \[!example\][^\n]*\n(?:>[^\n]*\n)*)?", sub, text)


def set_fields(text, values):
    m = FM.match(text)
    head = m.group(1)
    for k, v in values.items():
        s = str(v)
        quote = isinstance(v, list) or s == "" or ": " in s or s[:1] in '[{"#'
        val = json.dumps(v, ensure_ascii=False) if quote else s
        head = re.sub(rf"^{k}: .*$", lambda _: f"{k}: {val}", head, flags=re.M)
    return "---\n" + head + "\n---\n" + text[m.end():]


def tool_fields(P, n):
    t = P.docs[n].type
    def checked(e):
        ds = [d for tg, d, _ in P.checks.values() if tg == e]
        return max(ds).date().isoformat() if ds else ""
    if t in ("service", "module"):
        f = P.compliance(n)
        return {"compliance": f"{f['met']}/{f['total']}", "violated": f["violated"], "checked": checked(n)}
    if t == "system":
        f, r = P.compliance(n), P.rollup(n)
        return {"compliance": f"{f['met']}/{f['total']}", "rollup": f"{r['met']}/{r['total']}", "violated": r["violated"]}
    if t == "project":
        r = P.rollup(n)
        return {"rollup": f"{r['met']}/{r['total']}", "violated": r["violated"]}
    if t == "goal":
        m, k = P.goal_progress(n)
        d, j = P.goal_tasks(n)
        bs = P.goal_branches(n)
        return {"affects": [L(e) for e in P.affects(n)], "progress": f"{m}/{k}", "tasks": f"{d}/{j}",
                "landed": f"{sum(b['state'] == 'landed' for b in bs.values())}/{len(bs)}" if bs else ""}
    if t == "finding":
        f = P.finding[n]
        return {"status": f["status"], "contract": f["contract"], "resolution": f["resolution"], "resolved": f["resolved"]}
    return {}


# ---------- canvases ----------
GW, DOC_W, DOC_H, SUB_W, CARD_W, GAP = 820, 700, 260, 697, 637, 80
CARD_H = {"Contracts": 200, "Interfaces": 160, "Modules": 160, "Tasks": 120, "Clauses": 160, "Waiting on you": 200}


class Canvas:
    def __init__(self, P):
        self.P, self.nodes, self.ids = P, [], set()

    def nid(self, *parts):
        h = hashlib.md5("/".join(parts).encode()).hexdigest()[:16]
        assert h not in self.ids, parts
        self.ids.add(h)
        return h

    def card(self, ctx, name, x, y, w, h, color=None, subpath=None):
        n = {"id": self.nid(ctx, name, subpath or ""), "type": "file",
             "file": os.path.relpath(self.P.docs[name].path, os.path.dirname(os.path.dirname(self.P.folder))),
             "x": x, "y": y, "width": w, "height": h}
        if subpath:
            n["subpath"] = subpath
        if color:
            n["color"] = color
        self.nodes.append(n)

    def group(self, ctx, label, x, y, w, h):
        self.nodes.append({"id": self.nid(ctx, "group", label), "type": "group", "label": label, "x": x, "y": y, "width": w, "height": h})

    @staticmethod
    def sub_h(label, k):
        return 80 + k * (CARD_H[label] + 60)

    def subgroup(self, ctx, label, items, x, y, owner=None):
        h = self.sub_h(label, len(items))
        self.group(ctx, label, x, y, SUB_W, h)
        for i, it in enumerate(items):
            name, sub, color = it if isinstance(it, tuple) else (it, None, None)
            if label == "Contracts" and owner and self.P.contract_red(name, owner):
                color = "1"
            self.card(ctx + "/" + label, name, x + 20, y + 80 + i * (CARD_H[label] + 60), CARD_W, CARD_H[label], color, sub)
        return h

    def entity(self, ctx, name, x, y, parts, color=None, dry=False):
        cy, layout = y + 400, []
        for label, items in parts:
            if items:
                layout.append((label, items, cy))
                cy += self.sub_h(label, len(items)) + GAP
        h = (cy - GAP + 60 - y) if layout else 50 + DOC_H + 50
        if dry:
            return h
        self.group(ctx, name, x, y, GW, h)
        self.card(ctx, name, x + 40, y + 50, DOC_W, DOC_H, color)
        for label, items, sy in layout:
            self.subgroup(ctx + "/" + name, label, items, x + 43, sy, owner=name)
        return h

    def rows(self, ctx, items, x, y, per_row=4, dry=False):
        w = h = 0
        for r in range(0, len(items), per_row):
            row = items[r:r + per_row]
            rh = max(self.entity(ctx, n, 0, 0, p, dry=True) for n, p, c in row)
            if not dry:
                for i, (n, p, c) in enumerate(row):
                    self.entity(ctx, n, x + i * (GW + GAP), y + h, p, c)
            w = max(w, len(row) * GW + (len(row) - 1) * GAP)
            h += rh + GAP
        return w, h - GAP

    def save(self, path):
        groups = sorted([n for n in self.nodes if n["type"] == "group"], key=lambda g: -(g["width"] * g["height"]))
        files = [n for n in self.nodes if n["type"] != "group"]
        json.dump({"nodes": groups + files, "edges": []}, open(path, "w"), indent="\t")
        return len(groups), len(files)


def service_parts(P, s):
    return [("Contracts", P.adopts(s)), ("Interfaces", names(P.fm(s, "interfaces"))), ("Modules", names(P.fm(s, "uses")))]


def project_canvas(P):
    C = Canvas(P)
    p = P.project
    blocks = []
    for sy in P.of("system"):
        items = [(s, service_parts(P, s), P.color(s)) for s in P.members(sy)]
        w, h = C.rows("", items, 0, 0, dry=True)
        own = P.adopts(sy)
        header = max(50 + DOC_H, 60 + C.sub_h("Contracts", len(own)) if own else 0)
        blocks.append((sy, items, own, header, max(w, DOC_W + (SUB_W + 80 if own else 0)) + 80, header + 100 + h + 60))
    p_ad = P.adopts(p)
    header = max(50 + DOC_H, 60 + C.sub_h("Contracts", len(p_ad)))
    x, y = 40, header + 120
    for sy, items, own, hh, w, h in blocks:
        C.group("sys", sy, x, y, w, h)
        C.card("sys/" + sy, sy, x + 40, y + 50, DOC_W, DOC_H, P.color(sy))
        if own:
            C.subgroup("sys/" + sy, "Contracts", own, x + 40 + DOC_W + 80, y + 60, owner=sy)
        C.rows("sys/" + sy, items, x + 40, y + hh + 100)
        x += w + 120
    my = y + max(b[5] for b in blocks) + 160
    mods = [(m, [("Contracts", P.adopts(m))], P.color(m)) for m in P.of("module")]
    mw, mh = C.rows("mod", mods, 0, 0, dry=True)
    C.group("prj", "Modules", 40, my - 20, mw + 80, mh + 100)
    C.rows("mod", mods, 80, my + 40)
    C.group("prj", p, 0, 0, max(x - 120 + 40, mw + 160), my + mh + 140)
    C.card("prj", p, 40, 50, DOC_W, DOC_H, P.color(p))
    C.subgroup("prj/" + p, "Contracts", p_ad, 40 + DOC_W + 80, 60, owner=p)
    return C


def goal_canvas(P, g):
    """A goal's footprint: the goal, its clauses, and each entity colored by the goal's obligations on it."""
    C = Canvas(P)
    obs = P.goal_obligations(g)
    keys, targets = [], []
    for o in obs:
        if (o.clause.owner, o.clause.id) not in keys:
            keys.append((o.clause.owner, o.clause.id))
        if o.entity not in targets:
            targets.append(o.entity)
    for t in P.tasks[g]:
        if t["target"] not in targets and t["target"] in P.docs:
            targets.append(t["target"])

    def gcolor(e):
        st = ["met" if P.goal_met(o) else "violated" if o.status == "violated" else "unverified" for o in obs if o.entity == e]
        return None if not st else "1" if "violated" in st else "3" if "unverified" in st else "4"
    C.card("goal", g, 40, 50, DOC_W, DOC_H, "6")
    h1 = C.subgroup("goal", "Clauses", [(k[0], "#" + k[1], None) for k in keys], 40 + DOC_W + 80, 60)
    top = max(50 + DOC_H, 60 + h1) + 100
    C.group("goal", "Footprint", 40, top, 4 * (DOC_W + 60) + 20, 80 + ((len(targets) + 3) // 4) * (DOC_H + 60))
    for i, e in enumerate(targets):
        C.card("foot", e, 60 + (i % 4) * (DOC_W + 60), top + 80 + (i // 4) * (DOC_H + 60), DOC_W, DOC_H, gcolor(e))
    fh = 80 + ((len(targets) + 3) // 4) * (DOC_H + 60)
    C.group("goal", g, 0, 0, max(4 * (DOC_W + 60) + 100, DOC_W + SUB_W + 200), top + fh + 60)
    return C


def tasks_canvas(P):
    """Open work across all goals, in lanes. A view only."""
    C = Canvas(P)
    lanes = {"Waiting on you": [], "Stuck": [], "Running": [], "Ready": [], "Landing": []}
    claims = {(c["goal"], c["task"]): c for c in P.claims()}
    for q in P.of("question"):
        blocked = [t for g in P.of("goal") if P.live(g) for t in P.tasks[g] if q in names(t["f"].get("blocked", "")) and t["box"] == " "]
        if P.fm(q, "status") == "open" and blocked:
            lanes["Waiting on you"].append(("q", q, blocked))
    for g in P.of("goal"):
        if not P.live(g):
            continue
        for t in P.tasks[g]:
            if t["box"] != " ":
                continue
            if int(t["f"].get("attempts", 0) or 0) >= 2:
                lanes["Stuck"].append(("t", t))
            elif (g, t["id"]) in claims:
                lanes["Running"].append(("run", t, claims[(g, t["id"])]))
        lanes["Ready"] += [("t", t) for t in P.ready(g)]
        if P.fm(g, "standing"):
            lanes["Landing"] += [("t", t) for t in ready_to_land(P, g)]
        elif ready_to_land(P, g):
            lanes["Landing"].append(("goal", g))
    LW, TH, SH = 760, 140, 200
    x = 0
    for lane, items in lanes.items():
        if not items:
            continue
        y, parts = 80, []
        for it in items:
            if it[0] == "q":
                h = 60 + SH + len(it[2]) * (TH + 20) + 40
                parts.append((it, y, h))
            elif it[0] == "run":
                h = 60 + TH + 20 + SH + 40
                parts.append((it, y, h))
            else:
                h = TH if it[0] == "t" else SH
                parts.append((it, y, h))
            y += h + 40
        C.group("lanes", lane, x, 0, LW, y + 20)
        for it, iy, h in parts:
            if it[0] == "q":
                label = "Blocks " + ", ".join(t["id"] for t in it[2])
                C.group("q", label + it[1], x + 20, iy, LW - 40, h)
                C.nodes[-1]["label"] = label
                C.card("q/" + it[1], it[1], x + 40, iy + 40, LW - 80, SH, "2")
                for k, t in enumerate(it[2]):
                    C.card("q/" + it[1], t["goal"], x + 40, iy + 60 + SH + k * (TH + 20), LW - 80, TH, None, "#^" + t["block"])
            elif it[0] == "run":
                t, c = it[1], it[2]
                label = f"{t['id']} · {c['repo']}"
                C.group("run", label, x + 20, iy, LW - 40, h)
                C.card("run/" + label, t["goal"], x + 40, iy + 40, LW - 80, TH, "6", "#^" + t["block"])
                C.card("run/" + label, c["log"], x + 40, iy + 60 + TH, LW - 80, SH)
            elif it[0] == "t":
                C.card("lane/" + lane, it[1]["goal"], x + 20, iy, LW - 40, TH, "4" if lane == "Landing" else "1" if lane == "Stuck" else None, "#^" + it[1]["block"])
            else:
                C.card("lane/" + lane, it[1], x + 20, iy, LW - 40, SH, "4")
        x += LW + 80
    return C


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("folder")
    ap.add_argument("--check", action="store_true")
    a = ap.parse_args()
    P = Project(a.folder)
    for r, e, m in P.validation:
        print(f"validation {r}: {m}")
    for r, e, m in P.lint:
        print("lint {}: {}".format(r, re.sub(r"\[\[|\]\]", "", m)))
    if a.check:
        sys.exit(1 if P.validation else 0)
    for n, d in P.docs.items():
        text = render_doc(P, d)
        tf = tool_fields(P, n)
        if tf:
            text = set_fields(text, tf)
        if text != open(d.path).read():
            open(d.path, "w").write(text)
    print("{}.canvas: {} groups, {} cards".format(P.project, *project_canvas(P).save(os.path.join(P.folder, f"{P.project}.canvas"))))
    os.makedirs(os.path.join(P.folder, "Canvases"), exist_ok=True)
    for old in glob.glob(os.path.join(P.folder, "Canvases", "*.canvas")):
        os.remove(old)
    print("Tasks.canvas: {} groups, {} cards".format(*tasks_canvas(P).save(os.path.join(P.folder, "Tasks.canvas"))))
    for goal in P.of("goal"):
        if P.goal_status(goal) not in ("done", "dropped") and not P.fm(goal, "standing"):
            print("Canvases/{}.canvas: {} groups, {} cards".format(goal, *goal_canvas(P, goal).save(os.path.join(P.folder, "Canvases", f"{goal}.canvas"))))


if __name__ == "__main__":
    main()
