import { ItemView, Setting, WorkspaceLeaf } from "obsidian";
import { docName } from "./logic";
import type CartographerPlugin from "./main";

export const PANEL = "cartographer-panel";

/** The side panel: the project's dashboard link, the New commands, and the user's tools. */
export class Panel extends ItemView {
	constructor(
		leaf: WorkspaceLeaf,
		private plugin: CartographerPlugin,
	) {
		super(leaf);
	}

	getViewType(): string {
		return PANEL;
	}

	getDisplayText(): string {
		return "Cartographer";
	}

	getIcon(): string {
		return "map";
	}

	async onOpen(): Promise<void> {
		this.render();
		this.registerEvent(this.app.metadataCache.on("resolved", () => this.render()));
	}

	render(): void {
		const el = this.contentEl;
		el.empty();
		el.addClass("carto-panel");
		const projects = this.plugin.projects();
		if (projects.length === 0) {
			el.createEl("p", { text: "No Cartographer project in this vault. Ask your agent to map your repositories (the carto-map skill)." });
			return;
		}
		for (const project of projects) {
			const name = docName(project.path);
			const folder = project.parent?.path ?? "";
			el.createEl("h4", { text: name });
			new Setting(el).setName("Dashboard").addButton((b) => b.setButtonText("Open").onClick(() => void this.app.workspace.getLeaf(false).openFile(project)));
			const kinds = ["system", "service", "module", "interface", "contract"];
			const row = new Setting(el).setName("New");
			for (const k of kinds) row.addButton((b) => b.setButtonText(k).onClick(() => void this.plugin.newEntity(folder, k)));
			new Setting(el)
				.setName("Undo last act")
				.setDesc("Takes back your last act, or the last approved change, if nothing was built on it since.")
				.addButton((b) => b.setButtonText("Undo").onClick(() => void this.plugin.act(folder, ["undo"], "Undone")));
			new Setting(el)
				.setName("Run now")
				.setDesc("Starts agents for the ready work. The runner ends by itself when only work for you is left.")
				.addButton((b) => b.setButtonText("Run").onClick(() => void this.plugin.runNow(folder)));
			new Setting(el)
				.setName("Lay out")
				.setDesc("Writes the computed fields and every canvas again.")
				.addButton((b) => b.setButtonText("Lay out").onClick(() => void this.plugin.act(folder, ["render"], "Laid out")));
		}
	}
}
