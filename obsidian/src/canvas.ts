import { Menu, Notice, TFile, WorkspaceLeaf } from "obsidian";
import { runJSON } from "./cli";
import { docName } from "./logic";
import type CartographerPlugin from "./main";
import { askPick } from "./modals";

interface GestureOut {
	change: string;
	summary: string[];
	notes: string[];
	offers: { contract: string; entity: string; because: string }[];
}

// The parts of Obsidian's canvas that the plugin uses. They are not in the public API.
interface CanvasNode {
	file?: TFile;
	color?: string;
	nodeEl: HTMLElement;
}
interface Canvas {
	nodes: Map<string, CanvasNode>;
	selection: Set<CanvasNode>;
}
interface CanvasView {
	file: TFile | null;
	canvas?: Canvas;
	data?: string;
	setViewData(data: string, clear: boolean): void;
}

const QUIET = 700;

/**
 * Reads the user's edits of a canvas as acts (carto gesture), highlights the cards of the
 * selected documents, and adds Cartographer's items to a card's menu.
 */
export class CanvasSupport {
	/** The text of each canvas as Cartographer last wrote it, to skip its own writes. */
	private written = new Map<string, string>();
	private timers = new Map<string, number>();
	private busy = new Set<string>();

	constructor(private plugin: CartographerPlugin) {}

	load(): void {
		const { app } = this.plugin;
		this.plugin.registerEvent(app.vault.on("modify", (f) => f instanceof TFile && f.extension === "canvas" && this.changed(f)));
		this.plugin.registerDomEvent(document, "pointerup", () => window.requestAnimationFrame(() => this.highlight()));
		this.plugin.registerDomEvent(document, "keyup", () => window.requestAnimationFrame(() => this.highlight()));
		// "canvas:node-menu" is not in the typed events.
		const ws = app.workspace as unknown as { on(name: string, cb: (menu: Menu, node: CanvasNode) => void): ReturnType<typeof app.workspace.on> };
		this.plugin.registerEvent(ws.on("canvas:node-menu", (menu, node) => this.menu(menu, node)));
	}

	private changed(file: TFile): void {
		if (this.plugin.projectOf(file.path) === null) return;
		window.clearTimeout(this.timers.get(file.path));
		this.timers.set(file.path, window.setTimeout(() => void this.read(file), QUIET));
	}

	private async read(file: TFile): Promise<void> {
		const project = this.plugin.projectOf(file.path);
		if (project === null || this.busy.has(file.path)) return;
		const { adapter } = this.plugin.app.vault;
		const text = await adapter.read(file.path);
		if (this.written.get(file.path) === text) return;
		this.busy.add(file.path);
		try {
			const out = await runJSON<GestureOut>(this.plugin.bin(), this.plugin.abs(project), ["gesture", this.plugin.abs(file.path)]);
			const next = await adapter.read(file.path);
			this.written.set(file.path, next);
			this.show(file, next);
			if (out.summary.length > 0) new Notice(`Cartographer: ${out.summary.join("; ").replace(/\*\*|\[\[|\]\]/g, "")}. Undo last act takes it back.`);
			for (const n of out.notes) new Notice(`Cartographer: ${n}`);
			await this.offer(project, out.offers);
			this.plugin.refreshBlocks();
		} catch (e) {
			new Notice(`Cartographer: ${(e as Error).message}`);
		} finally {
			this.busy.delete(file.path);
		}
	}

	/** Shows the new layout in each open view of the canvas. */
	private show(file: TFile, text: string): void {
		this.plugin.app.workspace.getLeavesOfType("canvas").forEach((leaf: WorkspaceLeaf) => {
			const view = leaf.view as unknown as CanvasView;
			if (view.file?.path === file.path && view.data !== text) view.setViewData(text, false);
		});
	}

	private async offer(project: string, offers: GestureOut["offers"]): Promise<void> {
		for (const entity of [...new Set(offers.map((o) => o.entity))]) {
			const mine = offers.filter((o) => o.entity === entity);
			const picked = await askPick(
				this.plugin.app,
				`Adopt for ${entity}?`,
				`${entity} now attaches ${[...new Set(mine.map((o) => o.because))].join(", ")}, which names these schema contracts.`,
				mine.map((o) => o.contract),
				true,
			);
			if (picked && picked.length > 0) {
				const ops = picked.map((c) => ({ op: "add", doc: entity, field: "adopts", value: `[[${c}]]` }));
				await this.plugin.actOps(project, `adopt ${picked.join(", ")} for ${entity}`, ops);
			}
		}
	}

	private activeCanvas(): CanvasView | null {
		const view = this.plugin.app.workspace.getMostRecentLeaf()?.view;
		return view?.getViewType() === "canvas" && (view as unknown as CanvasView).canvas ? (view as unknown as CanvasView) : null;
	}

	/** Highlights every card of the documents of the selected cards. */
	private highlight(): void {
		const view = this.activeCanvas();
		if (!view?.canvas) return;
		const files = new Set([...view.canvas.selection].map((n) => n.file?.path).filter((p): p is string => !!p));
		for (const node of view.canvas.nodes.values()) {
			node.nodeEl.toggleClass("carto-same", !!node.file && files.has(node.file.path) && !view.canvas.selection.has(node));
		}
	}

	private menu(menu: Menu, node: CanvasNode): void {
		const file = node.file;
		if (!file || file.extension !== "md") return;
		const project = this.plugin.projectOf(file.path);
		if (project === null) return;
		const type = this.plugin.typeOf(file);
		const name = docName(file.path);
		const open = (sub: string) => void this.plugin.app.workspace.openLinkText(`${name}#${sub}`, file.path);
		if (["service", "module", "system", "project"].includes(type)) {
			menu.addItem((i) => i.setTitle("Show obligations").setIcon("list-checks").onClick(() => open("Obligations")));
			if (node.color === "1") {
				menu.addItem((i) => i.setTitle("Open findings").setIcon("alert-triangle").onClick(() => this.findings(project, name)));
			}
		}
		if (type === "contract") {
			menu.addItem((i) => i.setTitle("Show adopters").setIcon("users").onClick(() => open("Adopters")));
			menu.addItem((i) => i.setTitle("Adopt for…").setIcon("plus").onClick(() => void this.adoptFor(project, name)));
		}
	}

	private findings(project: string, entity: string): void {
		const open = this.plugin.docsOfType("finding", project).filter((f) => {
			const fm = this.plugin.app.metadataCache.getFileCache(f)?.frontmatter;
			return fm?.status === "open" && String(fm?.entity ?? "").replace(/\[\[|\]\]/g, "") === entity;
		});
		if (open.length === 0) {
			new Notice(`Cartographer: ${entity} has no open finding.`);
			return;
		}
		for (const f of open) void this.plugin.app.workspace.getLeaf("tab").openFile(f);
	}

	private async adoptFor(project: string, contract: string): Promise<void> {
		const entities = ["project", "system", "service", "module"].flatMap((t) => this.plugin.docsOfType(t, project)).filter((f) => {
			const adopts = this.plugin.app.metadataCache.getFileCache(f)?.frontmatter?.adopts;
			return !(Array.isArray(adopts) && adopts.some((a: unknown) => String(a) === `[[${contract}]]`));
		});
		const picked = await askPick(this.plugin.app, `Adopt ${contract} for…`, "Each entity you pick adopts the contract, in one act.", entities.map((f) => docName(f.path)), false);
		if (!picked || picked.length === 0) return;
		await this.plugin.actOps(project, `adopt ${contract} for ${picked.join(", ")}`, picked.map((e) => ({ op: "add", doc: e, field: "adopts", value: `[[${contract}]]` })));
	}
}
