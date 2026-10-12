import { writeFile } from "node:fs/promises";
import path from "node:path";
import { afterAll, beforeAll, describe, expect, it } from "vitest";
import { buildCarto, type Carto, launchObsidian, type ObsidianInstance, until } from "./harness";

const TIMEOUT = 60_000;
const QUESTION = "Should invoice-worker adopt the Shop Data Schema";

// One Obsidian runs the whole story; each test starts from the state the one before left.
describe("Cartographer in Obsidian", () => {
	let bin: Carto;
	let o: ObsidianInstance;

	beforeAll(async () => {
		bin = await buildCarto();
		o = await launchObsidian(bin);
	}, 120_000);

	afterAll(async () => {
		await o?.close();
		await bin?.remove();
	});

	async function open(file: string, mode: "preview" | "source" = "preview"): Promise<void> {
		await until(`Obsidian to see ${file}`, () => o.page.evaluate((f) => !!(window as any).app.vault.getFileByPath(f), file));
		await o.page.evaluate(
			async ({ file, mode }) => {
				const app = (window as any).app;
				await app.workspace.getLeaf(false).openFile(app.vault.getFileByPath(file), { state: { mode } });
			},
			{ file, mode },
		);
	}

	/** Scrolls the open document to a heading, so that reading view renders its section. */
	async function scrollTo(heading: string): Promise<void> {
		await o.page.evaluate(async (h) => {
			const app = (window as any).app;
			const view = app.workspace.getMostRecentLeaf().view;
			const line = (await app.vault.read(view.file)).split("\n").indexOf(h);
			view.currentMode.applyScroll(line);
		}, heading);
	}

	/** Whether the vault has nothing that is not committed, apart from Obsidian's own settings. */
	const clean = async () => (await o.git(["--no-optional-locks", "status", "--porcelain", "--", ".", ":!.obsidian"])) === "";

	/** The text of each Attention line on the open page. */
	const lines = () => o.page.$$eval(".carto-block .carto-item .carto-item-text", (els) => els.map((e) => e.textContent ?? ""));

	/** Clicks the button with a label on the Attention line that contains text. */
	async function click(line: string, label: string): Promise<void> {
		// A block can render again between the lookup and the click; then the lookup runs again.
		await until(`the button ${label} on “${line}”`, async () => {
			for (const item of await o.page.$$(".carto-block .carto-item")) {
				if (!((await item.textContent().catch(() => "")) ?? "").includes(line)) continue;
				for (const b of await item.$$("button")) {
					if ((await b.textContent().catch(() => "")) === label) return b.click({ timeout: 1000 }).then(() => true, () => undefined);
				}
			}
			return undefined;
		});
	}

	it("renders the dashboard's decisions with their buttons", async () => {
		await open("MyShop/MyShop.md");
		await scrollTo("## Attention");
		const got = await until("the Attention lines", async () => ((await lines()).length >= 4 ? lines() : undefined));
		expect(got.join("\n")).toContain("Land T2 of Upkeep");
		expect(got.join("\n")).toContain(`Answer ${QUESTION}`);
		const labels = await o.page.$$eval(".carto-block .carto-item button", (bs) => bs.map((b) => b.textContent));
		expect(labels).toEqual(expect.arrayContaining(["Land", "Request changes", "Accept recommended", "Not now until…", "1. Adopt ★"]));
		await scrollTo("## Goals");
		await until("Run now", () => o.page.$("button:text-is('Run now')").then((b) => b ?? undefined));
		await scrollTo("## Since your last visit");
		await until("Mark seen", () => o.page.$("button:text-is('Mark seen')").then((b) => b ?? undefined));
		expect(await o.page.textContent(".block-language-cartographer-since")).toContain("Since the project started.");
		await scrollTo("## Attention");
		expect(await o.page.$$eval(".carto-block", (els) => els.map((e) => e.textContent ?? "").join(" "))).not.toContain("**Mark seen**");
	}, TIMEOUT);

	it("answers a question with Accept recommended", async () => {
		await click(QUESTION, "Accept recommended");
		await until("the question to leave Attention", async () => !(await lines()).some((l) => l.includes(QUESTION)));
		const q = await o.read(`MyShop/Questions/${QUESTION}.md`);
		expect(q).toContain("status: answered");
		expect(q).toContain("[chosen:: 1]");
		expect(q).toContain("[apply:: pending]");
		await until("the commit", clean);
	}, TIMEOUT);

	it("hides a line with Not now until…", async () => {
		await click("How do we prove API-1", "Not now until…");
		const tomorrow = await until("the date dialog", () => o.page.$(".modal button:text-is('Tomorrow')").then((b) => b ?? undefined));
		await tomorrow.click();
		await until("the line to leave Attention", async () => !(await lines()).some((l) => l.includes("How do we prove API-1")));
		// The example's draft goal was deferred already, until 2026-11-01.
		expect(await o.page.textContent(".carto-block .carto-hint")).toMatch(/^Deferred: 2 \(until \d{4}-\d{2}-\d{2}, until 2026-11-01\)\.$|^Deferred: 2 \(until 2026-11-01, until \d{4}-\d{2}-\d{2}\)\.$/);
		expect(await o.read("MyShop/Questions/How do we prove API-1.md")).toMatch(/^defer: \d{4}-\d{2}-\d{2}$/m);
	}, TIMEOUT);

	it("approves an agent's proposal from Attention", async () => {
		const ops = path.join(o.vault, "..", "ops.json");
		await writeFile(ops, JSON.stringify([{ op: "add", doc: "shop-web", field: "adopts", value: "[[API Standards]]" }]));
		await o.carto(["propose", "--title", "shop-web adopts API Standards", "--ops", ops]);
		await o.page.evaluate(() => (window as any).app.plugins.plugins.cartographer.refreshBlocks());
		await click("Approve the change", "Approve");
		await until("the adoption", async () => (await o.read("MyShop/Services/shop-web.md")).includes("[[API Standards]]"));
		await until("the line to leave Attention", async () => !(await lines()).some((l) => l.includes("Approve the change")));
		await until("the commit", clean);
		await o.page.evaluate(() => (window as any).app.commands.executeCommandById("cartographer:undo"));
		await until("the undo", async () => !(await o.read("MyShop/Services/shop-web.md")).includes("[[API Standards]]"));
	}, TIMEOUT);

	it("reads a card dropped on the canvas as an adoption, and shows the new layout", async () => {
		await open("MyShop/MyShop.canvas");
		const before = await until("the canvas", () =>
			o.page.evaluate(() => {
				const view = (window as any).app.workspace.getMostRecentLeaf()?.view;
				return view?.canvas?.nodes?.size > 0 ? view.canvas.nodes.size : undefined;
			}),
		);
		// A drop from the file list: a new card of the contract inside shop-web's Contracts subgroup.
		await o.page.evaluate(() => {
			const app = (window as any).app;
			const canvas = app.workspace.getMostRecentLeaf().view.canvas;
			const nodes = [...canvas.nodes.values()];
			const inside = (g: any, n: any) => n.x >= g.x && n.y >= g.y && n.x + n.width <= g.x + g.width && n.y + n.height <= g.y + g.height;
			const web = nodes.find((n: any) => n.label === "shop-web");
			const sub = nodes.find((n: any) => n.label === "Contracts" && inside(web, n));
			canvas.createFileNode({ file: app.vault.getFileByPath("MyShop/Contracts/API Standards.md"), pos: { x: sub.x + 40, y: sub.y + 60 }, size: { width: 400, height: 160 }, save: true, focus: false });
			canvas.requestSave();
		});
		await until("the adoption", async () => (await o.read("MyShop/Services/shop-web.md")).includes("[[API Standards]]"), 20_000);
		// The open view shows the new layout: the dropped card is gone, and shop-web's subgroup holds a card of the contract.
		const after = await until("the view to show the layout", () =>
			o.page.evaluate(() => {
				const view = (window as any).app.workspace.getMostRecentLeaf().view;
				return view.data && !JSON.parse(view.data).nodes.some((n: any) => n.width === 400) ? view.canvas.nodes.size : undefined;
			}),
		);
		expect(after).toBe(before + 1);
		await until("the commit", clean);
		expect(await o.git(["log", "-1", "--format=%s"])).toContain("carto: canvas: add API Standards to shop-web adopts");
	}, TIMEOUT);

	it("takes the drop back with Undo last act", async () => {
		await o.page.evaluate(() => (window as any).app.commands.executeCommandById("cartographer:undo"));
		await until("the undo", async () => !(await o.read("MyShop/Services/shop-web.md")).includes("[[API Standards]]"));
		await until("the commit", clean);
	}, TIMEOUT);

	it("highlights every card of the selected document", async () => {
		const marked = await until("the highlight", () =>
			o.page.evaluate(() => {
				const canvas = (window as any).app.workspace.getMostRecentLeaf().view.canvas;
				const cards = [...canvas.nodes.values()].filter((n: any) => n.file?.path === "MyShop/Contracts/Shop Data Schema.md");
				canvas.deselectAll();
				canvas.select(cards[0]);
				document.dispatchEvent(new PointerEvent("pointerup"));
				return new Promise<number | undefined>((resolve) =>
					requestAnimationFrame(() => requestAnimationFrame(() => {
						const n = document.querySelectorAll(".canvas-node.carto-same").length;
						resolve(cards.length > 1 && n === cards.length - 1 ? n : undefined);
					})),
				);
			}),
		);
		expect(marked).toBeGreaterThan(0);
	}, TIMEOUT);

	it("makes a new service from the New service command", async () => {
		await open("MyShop/MyShop.md");
		await scrollTo("## Attention");
		await o.page.evaluate(() => (window as any).app.commands.executeCommandById("cartographer:new-service"));
		const modal = await until("the New service dialog", () => o.page.$(".modal").then((m) => m ?? undefined));
		const inputs = await modal.$$("input[type=text]");
		await inputs[0]!.fill("refund-worker");
		await inputs[1]!.fill("Pays refunds");
		await inputs[2]!.fill("github.com/myshop/myshop-api");
		await inputs[3]!.fill("refunds/");
		await (await modal.$("select"))!.selectOption("Storefront");
		await (await modal.$("button:text-is('Create')"))!.click();
		const text = await until("the new service", () => o.read("MyShop/Services/refund-worker.md").catch(() => undefined));
		expect(text).toContain("type: carto-service");
		expect(text).toContain('systems: ["[[Storefront]]"]');
		expect(await o.carto(["check"])).not.toContain("validation");
		await until("the commit", clean);
	}, TIMEOUT);

	it("commits an edit by hand after a pause", async () => {
		await o.page.evaluate(async () => {
			const app = (window as any).app;
			const file = app.vault.getFileByPath("MyShop/Systems/Storefront.md");
			await app.vault.process(file, (text: string) => text.replace("## Notes\n", "## Notes\n\nA note by hand.\n"));
		});
		await until("the edit on disk", () => o.read("MyShop/Systems/Storefront.md").then((t) => t.includes("A note by hand.")));
		expect(await o.git(["status", "--porcelain"])).toContain("Storefront.md");
		await o.page.evaluate(() => (window as any).app.plugins.plugins.cartographer.commitEdits.run());
		await until("the commit", clean);
		expect(await o.git(["log", "-1", "--format=%s"])).toContain("carto: 1 file edited by hand");
	}, TIMEOUT);

	it("logs no error in the console", () => {
		expect(o.errors).toEqual([]);
	});
});
