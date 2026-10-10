/** One option of a question, a finding, or a lint result, as `carto block attention --json` gives it. */
export interface Option {
	n: string;
	title: string;
	recommended?: boolean;
}

/** One decision of Attention, as `carto block attention --json` gives it. */
export interface Item {
	kind: "answer" | "decide" | "lint" | "approve-change" | "approve-goal" | "land" | "unstick" | "renew" | "do";
	key: string;
	doc?: string;
	text: string;
	options?: Option[];
	actions?: string[];
	entities?: string[];
	defer_key?: string;
	defer?: string;
}

/** What an action needs from the user before it runs. */
export type Need = { kind: "none" } | { kind: "text"; prompt: string } | { kind: "entity-text"; prompt: string; entities: string[] };

/** The input an action needs: a hint, a reason, or the changes and the entity they are on. */
export function need(item: Item, action: string): Need {
	switch (action) {
		case "Retry with a hint":
			return { kind: "text", prompt: "A hint for the agent" };
		case "Drop":
			return item.kind === "unstick" ? { kind: "text", prompt: "Why drop this task?" } : { kind: "none" };
		case "Request changes":
			return item.key.includes("#")
				? { kind: "text", prompt: "The changes you want before it lands" }
				: { kind: "entity-text", prompt: "The changes you want before it lands", entities: item.entities ?? [] };
	}
	return { kind: "none" };
}

/** The `carto` arguments of a button. text and entity are what need() asked the user for. */
export function actionArgs(item: Item, action: string, text = "", entity = ""): string[] {
	const words = (w: string) => ["--words", w];
	switch (item.kind) {
		case "approve-change":
			if (action === "Approve") return ["approve", item.key, ...words("approved in Obsidian")];
			if (action === "Reject") return ["reject", item.key, ...words("rejected in Obsidian")];
			break;
		case "approve-goal":
			if (action === "Approve") return ["approve", item.key];
			if (action === "Drop") return ["reject", item.key];
			break;
		case "land":
			if (action === "Land") return ["land", item.key];
			if (action === "Request changes") return ["request", item.key, ...words(text), ...(entity ? ["--on", entity] : [])];
			break;
		case "unstick":
			if (action === "Retry with a hint") return ["retry", item.key, ...words(text)];
			if (action === "I'll do it") return ["take", item.key];
			if (action === "Drop") return ["drop", item.key, ...words(text)];
			break;
	}
	throw new Error(`no action "${action}" on a ${item.kind} line`);
}

/** The `carto` arguments that choose option n of a question, a finding, or a lint result. */
export function chooseArgs(item: Item, n: string): string[] {
	switch (item.kind) {
		case "answer":
			return ["answer", item.key, n, "--words", `option ${n}, chosen in Obsidian`];
		case "decide":
			return ["choose", item.key, n, "--words", `option ${n}, chosen in Obsidian`];
		case "lint": {
			const [rule, entity] = item.key.split("|");
			return ["lint", rule ?? "", entity ?? "", n];
		}
	}
	throw new Error(`a ${item.kind} line has no options`);
}

/** The recommended option of a line, or null. */
export function recommended(item: Item): Option | null {
	return item.options?.find((o) => o.recommended) ?? null;
}

const pad = (n: number) => String(n).padStart(2, "0");
const day = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;

/**
 * Reads the date of "Not now until…": a date (2026-11-01), a count of days or weeks (3d,
 * 2w), or tomorrow and next week. It returns the date as yyyy-mm-dd, or null when the
 * text is no date or not in the future.
 */
export function parseUntil(text: string, now: Date): string | null {
	const t = text.trim().toLowerCase();
	const plus = (days: number) => {
		const d = new Date(now.getFullYear(), now.getMonth(), now.getDate() + days);
		return day(d);
	};
	if (t === "tomorrow") return plus(1);
	if (t === "next week") return plus(7);
	const rel = /^(\d+)\s*(d|day|days|w|week|weeks)$/.exec(t);
	if (rel) {
		const n = Number(rel[1]) * (rel[2]!.startsWith("w") ? 7 : 1);
		return n > 0 ? plus(n) : null;
	}
	const abs = /^(\d{4})-(\d{2})-(\d{2})$/.exec(t);
	if (abs) {
		const d = new Date(Number(abs[1]), Number(abs[2]) - 1, Number(abs[3]));
		if (day(d) !== t || day(d) <= day(now)) return null;
		return t;
	}
	return null;
}

/** The block kinds the plugin renders, each from `carto block <kind> <document>`. */
export const BLOCK_KINDS = ["since", "attention", "goals", "health", "obligations", "children", "work", "adopters", "attached", "progress"] as const;

/** Markdown of a block without the action words the plugin turns into buttons. */
export function withoutActions(markdown: string): string {
	return markdown.replace(/\s*\*\*(Mark seen|Run now)\*\*/g, "");
}

/**
 * The project folder of a vault path: the nearest folder, the path's own or a parent,
 * that holds a carto-project document. isProject answers for one folder.
 */
export function projectOf(path: string, isProject: (folder: string) => boolean): string | null {
	const parts = path.split("/");
	for (let i = parts.length - 1; i >= 1; i--) {
		const folder = parts.slice(0, i).join("/");
		if (isProject(folder)) return folder;
	}
	return isProject("") ? "" : null;
}

/** The document name of a vault path: its file name without .md. */
export function docName(path: string): string {
	return (path.split("/").pop() ?? path).replace(/\.md$/, "");
}

/** The fields the New command asks for each kind of entity. */
export const NEW_FIELDS: Record<string, string[]> = {
	system: ["description", "code"],
	service: ["description", "repository", "path", "system"],
	module: ["description", "repository", "path"],
	interface: ["description"],
	contract: ["description", "code"],
};

/** The `carto new` arguments for an entity. */
export function newArgs(kind: string, name: string, fields: Record<string, string>): string[] {
	const args = ["new", kind, name.trim()];
	for (const f of NEW_FIELDS[kind] ?? []) {
		const v = (fields[f] ?? "").trim();
		if (v) args.push(`--${f}`, v);
	}
	return args;
}
