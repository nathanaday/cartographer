import { MarkdownRenderChild, MarkdownRenderer, Notice } from "obsidian";
import { runJSON } from "./cli";
import { actionArgs, chooseArgs, docName, Item, need, recommended, withoutActions } from "./logic";
import type CartographerPlugin from "./main";
import { askText, askUntil } from "./modals";

interface BlockOut {
	markdown: string;
	items?: Item[];
	deferred?: Item[];
}

/** One rendered cartographer-* block. It renders again after every act and every change of the map. */
export class BlockView extends MarkdownRenderChild {
	private generation = 0;

	constructor(
		private plugin: CartographerPlugin,
		private kind: string,
		el: HTMLElement,
		private sourcePath: string,
	) {
		super(el);
	}

	onload(): void {
		this.plugin.blocks.add(this);
		void this.render();
	}

	onunload(): void {
		this.plugin.blocks.delete(this);
	}

	async render(): Promise<void> {
		const gen = ++this.generation;
		const project = this.plugin.projectOf(this.sourcePath);
		if (project === null) {
			this.containerEl.setText(`cartographer-${this.kind}: this document is in no Cartographer project.`);
			return;
		}
		let out: BlockOut;
		try {
			out = await runJSON<BlockOut>(this.plugin.bin(), this.plugin.abs(project), ["block", this.kind, docName(this.sourcePath)]);
		} catch (e) {
			if (gen === this.generation) this.containerEl.setText(`Cartographer: ${(e as Error).message}`);
			return;
		}
		if (gen !== this.generation) return;
		const el = this.containerEl;
		el.empty();
		el.addClass("carto-block");
		if (this.kind === "attention") {
			await this.attention(el, project, out.items ?? [], out.deferred ?? []);
			return;
		}
		await MarkdownRenderer.render(this.plugin.app, withoutActions(out.markdown), el.createDiv(), this.sourcePath, this);
		if (this.kind === "since") this.button(el, "Mark seen", () => this.plugin.act(project, ["seen"], "Marked seen"));
		if (this.kind === "goals") this.button(el, "Run now", async () => this.plugin.runNow(project));
	}

	private button(el: HTMLElement, label: string, onClick: () => Promise<unknown>, cta = false): HTMLButtonElement {
		const b = el.createEl("button", { text: label, cls: cta ? "mod-cta" : "" });
		b.addEventListener("click", async () => {
			const all = this.containerEl.querySelectorAll("button");
			all.forEach((x) => (x.disabled = true));
			try {
				await onClick();
			} finally {
				all.forEach((x) => (x.disabled = false));
			}
		});
		return b;
	}

	private async attention(el: HTMLElement, project: string, items: Item[], deferred: Item[]): Promise<void> {
		if (items.length === 0) el.createEl("p", { text: "Nothing waits for you." });
		for (const item of items) {
			const row = el.createDiv({ cls: "carto-item" });
			row.dataset.kind = item.kind;
			await MarkdownRenderer.render(this.plugin.app, item.text, row.createDiv({ cls: "carto-item-text" }), this.sourcePath, this);
			const acts = row.createDiv({ cls: "carto-actions" });
			for (const o of item.options ?? []) {
				this.button(acts, `${o.n}. ${o.title}${o.recommended ? " ★" : ""}`, () => this.plugin.act(project, chooseArgs(item, o.n), `Chose ${o.title}`));
			}
			const rec = recommended(item);
			if (rec) this.button(acts, "Accept recommended", () => this.plugin.act(project, chooseArgs(item, rec.n), `Chose ${rec.title}`), true);
			for (const a of item.actions ?? []) {
				this.button(acts, a, () => this.action(project, item, a), a === "Approve" || a === "Land");
			}
			if (item.defer_key) {
				const key = item.defer_key;
				this.button(acts, "Not now until…", async () => {
					const until = await askUntil(this.plugin.app);
					if (until) await this.plugin.act(project, ["defer", key, until], `Hidden until ${until}`);
				});
			}
		}
		if (deferred.length > 0) {
			el.createEl("p", { cls: "carto-hint", text: `Deferred: ${deferred.length} (${deferred.map((d) => "until " + d.defer).join(", ")}).` });
		}
	}

	private async action(project: string, item: Item, action: string): Promise<void> {
		const n = need(item, action);
		let text = "";
		let entity = "";
		if (n.kind !== "none") {
			const got = await askText(this.plugin.app, action, n.prompt, n.kind === "entity-text" ? n.entities : []);
			if (!got) return;
			({ text, entity } = got);
		}
		let args: string[];
		try {
			args = actionArgs(item, action, text, entity);
		} catch (e) {
			new Notice(`Cartographer: ${(e as Error).message}`);
			return;
		}
		await this.plugin.act(project, args, `${action}: done`);
	}
}
