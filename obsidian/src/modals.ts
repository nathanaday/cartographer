import { App, Modal, Setting } from "obsidian";
import { NEW_FIELDS, parseUntil } from "./logic";

/** Asks for a line of text, and an entity when entities are given. Resolves null on cancel. */
export function askText(app: App, title: string, prompt: string, entities: string[] = []): Promise<{ text: string; entity: string } | null> {
	return new Promise((resolve) => {
		const m = new (class extends Modal {
			text = "";
			entity = entities[0] ?? "";
			done = false;
			onOpen() {
				this.setTitle(title);
				if (entities.length > 0) {
					new Setting(this.contentEl).setName("On").addDropdown((d) => {
						for (const e of entities) d.addOption(e, e);
						d.setValue(this.entity).onChange((v) => (this.entity = v));
					});
				}
				new Setting(this.contentEl).setName(prompt).addTextArea((t) => {
					t.inputEl.rows = 3;
					t.inputEl.addClass("carto-wide");
					t.onChange((v) => (this.text = v));
					window.setTimeout(() => t.inputEl.focus(), 0);
				});
				new Setting(this.contentEl).addButton((b) =>
					b.setButtonText("OK").setCta().onClick(() => {
						if (this.text.trim() === "") return;
						this.done = true;
						this.close();
					}),
				);
			}
			onClose() {
				this.contentEl.empty();
				resolve(this.done ? { text: this.text.trim(), entity: this.entity } : null);
			}
		})(app);
		m.open();
	});
}

/** Asks for the date of Not now until…. Resolves yyyy-mm-dd, or null on cancel. */
export function askUntil(app: App): Promise<string | null> {
	return new Promise((resolve) => {
		const m = new (class extends Modal {
			value: string | null = null;
			onOpen() {
				this.setTitle("Not now until…");
				const hint = this.contentEl.createEl("p", { cls: "carto-hint", text: "A date (2026-11-01), days or weeks (3d, 2w), tomorrow, or next week." });
				const pick = (text: string) => {
					const d = parseUntil(text, new Date());
					if (!d) {
						hint.setText(`“${text}” is no date in the future.`);
						hint.addClass("carto-error");
						return;
					}
					this.value = d;
					this.close();
				};
				new Setting(this.contentEl)
					.addButton((b) => b.setButtonText("Tomorrow").onClick(() => pick("tomorrow")))
					.addButton((b) => b.setButtonText("Next week").onClick(() => pick("next week")))
					.addButton((b) => b.setButtonText("In a month").onClick(() => pick("30d")));
				let typed = "";
				new Setting(this.contentEl)
					.setName("Until")
					.addText((t) => {
						t.setPlaceholder("2026-11-01");
						t.onChange((v) => (typed = v));
						t.inputEl.addEventListener("keydown", (e) => e.key === "Enter" && pick(typed));
					})
					.addButton((b) => b.setButtonText("OK").setCta().onClick(() => pick(typed)));
			}
			onClose() {
				this.contentEl.empty();
				resolve(this.value);
			}
		})(app);
		m.open();
	});
}

const LABELS: Record<string, string> = {
	description: "Description (one sentence)",
	code: "Code (for clause numbers, such as SEC)",
	repository: "Repository (github.com/org/repo)",
	path: "Path in the repository (server/)",
	system: "System",
};

/** Asks for the name and the fields of a new entity. Resolves null on cancel. */
export function askNew(app: App, kind: string, systems: string[]): Promise<{ name: string; fields: Record<string, string> } | null> {
	return new Promise((resolve) => {
		const m = new (class extends Modal {
			name = "";
			fields: Record<string, string> = { system: systems[0] ?? "" };
			done = false;
			onOpen() {
				this.setTitle(`New ${kind}`);
				new Setting(this.contentEl).setName("Name").addText((t) => {
					t.onChange((v) => (this.name = v));
					window.setTimeout(() => t.inputEl.focus(), 0);
				});
				for (const f of NEW_FIELDS[kind] ?? []) {
					const s = new Setting(this.contentEl).setName(LABELS[f] ?? f);
					if (f === "system") {
						s.addDropdown((d) => {
							d.addOption("", "none");
							for (const sy of systems) d.addOption(sy, sy);
							d.setValue(this.fields.system ?? "").onChange((v) => (this.fields.system = v));
						});
					} else {
						s.addText((t) => t.onChange((v) => (this.fields[f] = v)));
					}
				}
				new Setting(this.contentEl).addButton((b) =>
					b.setButtonText("Create").setCta().onClick(() => {
						if (this.name.trim() === "") return;
						this.done = true;
						this.close();
					}),
				);
			}
			onClose() {
				this.contentEl.empty();
				resolve(this.done ? { name: this.name, fields: this.fields } : null);
			}
		})(app);
		m.open();
	});
}

/** Asks which of a list to take. Resolves the chosen ones, or null on cancel. */
export function askPick(app: App, title: string, text: string, choices: string[], checked: boolean): Promise<string[] | null> {
	return new Promise((resolve) => {
		const m = new (class extends Modal {
			picked = new Set(checked ? choices : []);
			done = false;
			onOpen() {
				this.setTitle(title);
				this.contentEl.createEl("p", { text });
				for (const c of choices) {
					new Setting(this.contentEl).setName(c).addToggle((t) => t.setValue(this.picked.has(c)).onChange((v) => (v ? this.picked.add(c) : this.picked.delete(c))));
				}
				new Setting(this.contentEl)
					.addButton((b) => b.setButtonText("Not now").onClick(() => this.close()))
					.addButton((b) =>
						b.setButtonText("OK").setCta().onClick(() => {
							this.done = true;
							this.close();
						}),
					);
			}
			onClose() {
				this.contentEl.empty();
				resolve(this.done ? choices.filter((c) => this.picked.has(c)) : null);
			}
		})(app);
		m.open();
	});
}
