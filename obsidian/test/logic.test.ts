import { strict as assert } from "node:assert";
import { test } from "node:test";
import { actionArgs, chooseArgs, deferArgs, docName, Item, need, newArgs, parseUntil, projectOf, recommended, withoutActions } from "../src/logic";

const answer: Item = {
	kind: "answer",
	key: "Should invoice-worker adopt the Shop Data Schema",
	text: "**Answer** …",
	options: [
		{ n: "1", title: "Adopt", recommended: true },
		{ n: "2", title: "Do not adopt" },
	],
	defer_key: "Should invoice-worker adopt the Shop Data Schema",
};

test("an option chooses through answer, choose, or lint", () => {
	assert.deepEqual(chooseArgs(answer, "2"), ["answer", answer.key, "2", "--words", "option 2, chosen in Obsidian"]);
	assert.deepEqual(chooseArgs({ ...answer, kind: "decide", key: "F" }, "1"), ["choose", "F", "1", "--words", "option 1, chosen in Obsidian"]);
	assert.deepEqual(chooseArgs({ kind: "lint", key: "L1|invoice-worker", text: "" }, "2"), ["lint", "L1", "invoice-worker", "2"]);
	const group: Item = { kind: "decide", key: "F1", text: "", members: ["F1", "F2"] };
	assert.deepEqual(chooseArgs(group, "3"), ["choose", "F1", "F2", "3", "--words", "option 3, chosen in Obsidian"]);
	assert.deepEqual(deferArgs(group, "2026-11-01"), ["defer", "F1", "F2", "2026-11-01"]);
	assert.deepEqual(deferArgs({ kind: "answer", key: "Q", text: "", defer_key: "Q" }, "2026-11-01"), ["defer", "Q", "2026-11-01"]);
	assert.equal(deferArgs({ kind: "approve-change", key: "C", text: "" }, "2026-11-01"), null);
	assert.equal(recommended(answer)?.n, "1");
	assert.equal(recommended({ ...answer, options: [{ n: "1", title: "x" }] }), null);
});

test("each button runs one user act", () => {
	const change: Item = { kind: "approve-change", key: "chg-abc123", text: "" };
	assert.deepEqual(actionArgs(change, "Approve"), ["approve", "chg-abc123", "--words", "approved in Obsidian"]);
	assert.deepEqual(actionArgs(change, "Reject"), ["reject", "chg-abc123", "--words", "rejected in Obsidian"]);
	const goal: Item = { kind: "approve-goal", key: "Add tax rates", text: "" };
	assert.deepEqual(actionArgs(goal, "Drop"), ["reject", "Add tax rates"]);
	const land: Item = { kind: "land", key: "Upkeep#T2", text: "" };
	assert.deepEqual(actionArgs(land, "Land"), ["land", "Upkeep#T2"]);
	assert.deepEqual(actionArgs(land, "Request changes", "Keep the column"), ["request", "Upkeep#T2", "--words", "Keep the column"]);
	const goalLand: Item = { kind: "land", key: "G", text: "", entities: ["shop-api"] };
	assert.deepEqual(actionArgs(goalLand, "Request changes", "Add a log", "shop-api"), ["request", "G", "--words", "Add a log", "--on", "shop-api"]);
	const stuck: Item = { kind: "unstick", key: "G#T5", text: "" };
	assert.deepEqual(actionArgs(stuck, "Retry with a hint", "Run npm ci"), ["retry", "G#T5", "--words", "Run npm ci"]);
	assert.deepEqual(actionArgs(stuck, "I'll do it"), ["take", "G#T5"]);
	assert.deepEqual(actionArgs(stuck, "Drop", "Done by hand"), ["drop", "G#T5", "--words", "Done by hand"]);
	assert.throws(() => actionArgs(stuck, "Land"));
});

test("an action asks only for what it needs", () => {
	assert.equal(need({ kind: "unstick", key: "G#T5", text: "" }, "Retry with a hint").kind, "text");
	assert.equal(need({ kind: "unstick", key: "G#T5", text: "" }, "Drop").kind, "text");
	assert.equal(need({ kind: "approve-goal", key: "G", text: "" }, "Drop").kind, "none");
	assert.equal(need({ kind: "land", key: "Upkeep#T2", text: "" }, "Request changes").kind, "text");
	const n = need({ kind: "land", key: "G", text: "", entities: ["a", "b"] }, "Request changes");
	assert.equal(n.kind, "entity-text");
	assert.deepEqual(n.kind === "entity-text" && n.entities, ["a", "b"]);
});

test("Not now reads dates, days, and weeks, only in the future", () => {
	const now = new Date(2026, 9, 9, 15, 0);
	assert.equal(parseUntil("tomorrow", now), "2026-10-10");
	assert.equal(parseUntil("next week", now), "2026-10-16");
	assert.equal(parseUntil("3d", now), "2026-10-12");
	assert.equal(parseUntil("2 weeks", now), "2026-10-23");
	assert.equal(parseUntil("30 days", now), "2026-11-08");
	assert.equal(parseUntil("2026-11-01", now), "2026-11-01");
	assert.equal(parseUntil("2026-10-09", now), null);
	assert.equal(parseUntil("2026-02-30", now), null);
	assert.equal(parseUntil("0d", now), null);
	assert.equal(parseUntil("soon", now), null);
});

test("the plugin finds the project of a path", () => {
	const projects = new Set(["Work/MyShop"]);
	const is = (f: string) => projects.has(f);
	assert.equal(projectOf("Work/MyShop/Services/shop-api.md", is), "Work/MyShop");
	assert.equal(projectOf("Work/MyShop/MyShop.md", is), "Work/MyShop");
	assert.equal(projectOf("Other/note.md", is), null);
	assert.equal(projectOf("MyShop.md", (f) => f === ""), "");
	assert.equal(docName("Work/MyShop/Services/shop-api.md"), "shop-api");
});

test("buttons replace their words in the markdown", () => {
	assert.equal(withoutActions("Since 2026-10-06 18:00. **Mark seen**"), "Since 2026-10-06 18:00.");
	assert.equal(withoutActions("Run queue: 1 ready task. **Run now**"), "Run queue: 1 ready task.");
});

test("New passes only the fields of its kind", () => {
	assert.deepEqual(newArgs("service", " refund-worker ", { description: "Pays refunds", repository: "r", path: "p/", system: "Payments", code: "X" }), [
		"new", "service", "refund-worker", "--description", "Pays refunds", "--repository", "r", "--path", "p/", "--system", "Payments",
	]);
	assert.deepEqual(newArgs("contract", "Refund Policy", { description: "Rules", code: "ref", path: "" }), ["new", "contract", "Refund Policy", "--description", "Rules", "--code", "ref"]);
});
