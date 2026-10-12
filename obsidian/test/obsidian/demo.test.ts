import { mkdir } from "node:fs/promises";
import path from "node:path";
import { afterAll, beforeAll, describe, it } from "vitest";
import { buildCarto, type Carto, launchObsidian, type ObsidianInstance, until } from "./harness";

// Screenshots of a copy of a real vault, to look at what the plugin shows. It runs only
// with DEMO_VAULT (the vault), DEMO_PROJECT (its project folder), DEMO_ROOTS (the folders
// of the clones, colon-separated), and DEMO_SHOTS (where the screenshots go).
const VAULT = process.env.DEMO_VAULT ?? "";
const SHOTS = process.env.DEMO_SHOTS ?? path.join(process.cwd(), "demo-shots");

describe.skipIf(!VAULT)("a demo vault in Obsidian", () => {
	let bin: Carto;
	let o: ObsidianInstance;
	const project = process.env.DEMO_PROJECT ?? "";

	beforeAll(async () => {
		await mkdir(SHOTS, { recursive: true });
		bin = await buildCarto();
		o = await launchObsidian(bin, { vault: VAULT, project, roots: (process.env.DEMO_ROOTS ?? "").split(":").filter((r) => r !== "") });
		await o.page.setViewportSize({ width: 1500, height: 1000 });
		// Load the plugin as Obsidian does when it opens the vault, with the console attached.
		await o.page.reload();
		await o.page.waitForFunction(() => (window as any).app?.workspace?.layoutReady, undefined, { timeout: 30_000 });
		await until("the plugin", () => o.page.evaluate(() => !!(window as any).app.plugins.plugins.cartographer?._loaded));
	}, 120_000);

	afterAll(async () => {
		if (o?.errors.length) console.log("console errors:\n" + o.errors.join("\n"));
		await o?.close();
		await bin?.remove();
	});

	async function open(file: string): Promise<void> {
		await until(`Obsidian to see ${file}`, () => o.page.evaluate((f) => !!(window as any).app.vault.getFileByPath(f), file));
		await o.page.evaluate(async (f) => {
			const app = (window as any).app;
			const leaf = app.workspace.getLeaf(false);
			await leaf.openFile(app.vault.getFileByPath(f), { state: { mode: "preview" } });
		}, file);
		await o.page.waitForTimeout(2500);
	}

	async function shot(name: string, heading?: string): Promise<void> {
		if (heading) {
			await o.page.evaluate((h) => {
				const view = (window as any).app.workspace.activeLeaf.view;
				const line = view.data.split("\n").findIndex((l: string) => l === h);
				view.currentMode.applyScroll(Math.max(0, line - 1));
			}, heading);
			await o.page.waitForTimeout(2000);
		}
		await o.page.screenshot({ path: path.join(SHOTS, `${name}.png`) });
	}

	it("shows the dashboard, a service, a contract, a finding, a goal, and the canvas", async () => {
		await open(`${project}/${project}.md`);
		await shot("1-dashboard-top");
		await shot("2-attention", "## Attention");
		await shot("3-goals", "## Goals");
		await shot("4-health", "## Health");
		for (const [name, file] of (process.env.DEMO_DOCS ?? "").split(";").filter((d) => d !== "").map((d, i) => [`${5 + i}-${path.basename(d, ".md")}`, d] as const)) {
			await open(`${project}/${file}`);
			await shot(name);
		}
		await open(`${project}/${project}.canvas`);
		await o.page.evaluate(() => (window as any).app.workspace.activeLeaf.view.canvas?.zoomToFit());
		await o.page.waitForTimeout(2000);
		await shot("9-canvas");
	}, 180_000);
});
