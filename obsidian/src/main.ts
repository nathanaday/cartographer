import { App, debounce, FileSystemAdapter, Notice, Plugin, PluginSettingTab, Setting, TFile, TFolder } from "obsidian";
import { tmpdir } from "os";
import { join } from "path";
import { writeFileSync, unlinkSync } from "fs";
import { BlockView } from "./blocks";
import { CanvasSupport } from "./canvas";
import { findBinary, run, startRunner } from "./cli";
import { BLOCK_KINDS, docName, newArgs, projectOf } from "./logic";
import { askNew } from "./modals";
import { Panel, PANEL } from "./panel";

interface Settings {
	binaryPath: string;
}

const DEFAULTS: Settings = { binaryPath: "" };

export default class CartographerPlugin extends Plugin {
	settings: Settings = DEFAULTS;
	blocks = new Set<BlockView>();
	private canvas = new CanvasSupport(this);
	/** The map changed outside the plugin, by an agent or the runner: render the blocks again. */
	private mapChanged = debounce(() => Date.now() > this.ownUntil && this.refreshBlocks(), 1500, true);
	/** Until this time, changes of the map come from the plugin's own act, which renders the blocks itself. */
	private ownUntil = 0;
	/** Projects with documents edited by hand, committed after a pause (Specification 6.4). */
	private edited = new Set<string>();
	private commitEdits = debounce(() => {
		const projects = [...this.edited];
		this.edited.clear();
		for (const p of projects) run(this.bin(), this.abs(p), ["snapshot"]).catch(() => undefined);
	}, 20000, true);

	async onload(): Promise<void> {
		this.settings = { ...DEFAULTS, ...((await this.loadData()) as Partial<Settings>) };
		for (const kind of BLOCK_KINDS) {
			this.registerMarkdownCodeBlockProcessor(`cartographer-${kind}`, (_src, el, ctx) => {
				ctx.addChild(new BlockView(this, kind, el, ctx.sourcePath));
			});
		}
		this.canvas.load();
		this.registerView(PANEL, (leaf) => new Panel(leaf, this));
		this.addRibbonIcon("map", "Cartographer", () => void this.openPanel());
		this.registerEvent(this.app.vault.on("modify", (f) => f instanceof TFile && f.extension === "md" && this.projectOf(f.path) !== null && this.mapChanged()));
		for (const event of ["modify", "create", "delete"] as const) {
			this.registerEvent(this.app.vault.on(event as "modify", (f) => this.noteEdit(f.path)));
		}
		this.registerEvent(this.app.vault.on("rename", (f, old) => (this.noteEdit(f.path), this.noteEdit(old))));

		this.addCommand({ id: "open-panel", name: "Open the side panel", callback: () => void this.openPanel() });
		this.addCommand({ id: "undo", name: "Undo last act", callback: () => this.withProject((p) => this.act(p, ["undo"], "Undone")) });
		this.addCommand({ id: "mark-seen", name: "Mark seen", callback: () => this.withProject((p) => this.act(p, ["seen"], "Marked seen")) });
		this.addCommand({ id: "run-now", name: "Run now", callback: () => this.withProject((p) => this.runNow(p)) });
		this.addCommand({ id: "lay-out", name: "Lay out the map again", callback: () => this.withProject((p) => this.act(p, ["render"], "Laid out")) });
		for (const kind of ["system", "service", "module", "interface", "contract"]) {
			this.addCommand({ id: `new-${kind}`, name: `New ${kind}`, callback: () => this.withProject((p) => this.newEntity(p, kind)) });
		}
		this.addSettingTab(new SettingsTab(this.app, this));
	}

	onunload(): void {
		this.commitEdits.run();
	}

	private noteEdit(path: string): void {
		if (!path.endsWith(".md")) return;
		const project = this.projectOf(path);
		if (project === null) return;
		this.edited.add(project);
		this.commitEdits();
	}

	bin(): string | null {
		return findBinary(this.settings.binaryPath);
	}

	/** The absolute path of a vault path. */
	abs(path: string): string {
		const base = (this.app.vault.adapter as FileSystemAdapter).getBasePath();
		return path === "" ? base : join(base, path);
	}

	typeOf(file: TFile): string {
		const t = this.app.metadataCache.getFileCache(file)?.frontmatter?.type;
		return typeof t === "string" && t.startsWith("carto-") ? t.slice(6) : "";
	}

	/** The project documents of the vault. */
	projects(): TFile[] {
		return this.app.vault.getMarkdownFiles().filter((f) => this.typeOf(f) === "project");
	}

	/** The project folder of a vault path, or null. */
	projectOf(path: string): string | null {
		return projectOf(path, (folder) => {
			const dir = folder === "" ? this.app.vault.getRoot() : this.app.vault.getFolderByPath(folder);
			return dir instanceof TFolder && dir.children.some((c) => c instanceof TFile && c.extension === "md" && this.typeOf(c) === "project");
		});
	}

	docsOfType(type: string, project: string): TFile[] {
		return this.app.vault.getMarkdownFiles().filter((f) => this.typeOf(f) === type && this.projectOf(f.path) === project);
	}

	/** Runs a command for the project of the active file, or the only project. */
	private withProject(f: (project: string) => unknown): void {
		const active = this.app.workspace.getActiveFile();
		const fromActive = active ? this.projectOf(active.path) : null;
		const all = this.projects();
		const project = fromActive ?? (all.length === 1 ? (all[0]!.parent?.path ?? "") : null);
		if (project === null) {
			new Notice(all.length === 0 ? "Cartographer: this vault has no project." : "Cartographer: open a document of the project first.");
			return;
		}
		void f(project);
	}

	/** Runs one of the user's acts, says what it did, and renders the blocks again. */
	async act(project: string, args: string[], done: string): Promise<boolean> {
		this.ownUntil = Number.POSITIVE_INFINITY;
		try {
			await run(this.bin(), this.abs(project), args);
			new Notice(`Cartographer: ${done}.`);
			return true;
		} catch (e) {
			new Notice(`Cartographer: ${(e as Error).message}`, 8000);
			return false;
		} finally {
			this.ownUntil = Date.now() + 2000;
			this.refreshBlocks();
		}
	}

	/** Applies declared operations at once as the user's act. */
	async actOps(project: string, title: string, ops: unknown[]): Promise<boolean> {
		const file = join(tmpdir(), `carto-ops-${Date.now()}.json`);
		writeFileSync(file, JSON.stringify(ops));
		try {
			return await this.act(project, ["act", "--title", title, "--ops", file], title);
		} finally {
			unlinkSync(file);
		}
	}

	async newEntity(project: string, kind: string): Promise<void> {
		const systems = this.docsOfType("system", project).map((f) => docName(f.path));
		const got = await askNew(this.app, kind, systems);
		if (!got) return;
		if (await this.act(project, newArgs(kind, got.name, got.fields), `New ${kind} ${got.name.trim()}`)) {
			const file = this.docsOfType(kind, project).find((f) => docName(f.path) === got.name.trim());
			if (file) await this.app.workspace.getLeaf(false).openFile(file);
		}
	}

	runNow(project: string): void {
		try {
			startRunner(this.bin(), this.abs(project), this.abs(""));
			new Notice("Cartographer: the runner started. Goals shows its agents; .carto/runner.log holds its output.");
		} catch (e) {
			new Notice(`Cartographer: ${(e as Error).message}`);
		}
	}

	refreshBlocks(): void {
		for (const b of this.blocks) void b.render();
	}

	private async openPanel(): Promise<void> {
		const existing = this.app.workspace.getLeavesOfType(PANEL)[0];
		const leaf = existing ?? this.app.workspace.getRightLeaf(false);
		if (!leaf) return;
		if (!existing) await leaf.setViewState({ type: PANEL, active: true });
		await this.app.workspace.revealLeaf(leaf);
	}
}

class SettingsTab extends PluginSettingTab {
	constructor(
		app: App,
		private plugin: CartographerPlugin,
	) {
		super(app, plugin);
	}

	display(): void {
		const el = this.containerEl;
		el.empty();
		const found = this.plugin.bin();
		new Setting(el)
			.setName("carto binary")
			.setDesc(found ? `In use: ${found}` : "Not found. Run make install in the cartographer repository, or give its path.")
			.addText((t) =>
				t
					.setPlaceholder("~/.cartographer/bin/carto")
					.setValue(this.plugin.settings.binaryPath)
					.onChange(async (v) => {
						this.plugin.settings.binaryPath = v;
						await this.plugin.saveData(this.plugin.settings);
					}),
			);
	}
}
