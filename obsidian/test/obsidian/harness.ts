import { type ChildProcess, execFile, spawn } from "node:child_process";
import { openSync } from "node:fs";
import { copyFile, cp, mkdir, mkdtemp, readdir, readFile, rm, stat, writeFile } from "node:fs/promises";
import { createServer } from "node:net";
import { homedir, tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { type Browser, chromium, type Page } from "playwright-core";

const OBSIDIAN = process.env.OBSIDIAN_BINARY ?? "/Applications/Obsidian.app/Contents/MacOS/Obsidian";
/** Obsidian's own data folder, where it keeps the app updates that it downloaded. */
const OBSIDIAN_DATA = process.env.OBSIDIAN_DATA ?? path.join(homedir(), "Library/Application Support/obsidian");
/** OBSIDIAN_SHOW=1 leaves the test window on screen, to watch a run. OBSIDIAN_LOG=file writes its console there, from the start. */
const SHOW = process.env.OBSIDIAN_SHOW === "1";
const PLUGIN = fileURLToPath(new URL("../../", import.meta.url));
const REPO = path.join(PLUGIN, "..");
const PLUGIN_FILES = ["main.js", "manifest.json", "styles.css"];

export interface Carto {
	path: string;
	remove(): Promise<void>;
}

/** Builds carto from this repository into a temporary folder. */
export async function buildCarto(): Promise<Carto> {
	const dir = await mkdtemp(path.join(tmpdir(), "carto-e2e-bin-"));
	const bin = path.join(dir, "carto");
	await run("go", ["build", "-o", bin, "./cmd/carto"], { cwd: REPO });
	return { path: bin, remove: () => rm(dir, { recursive: true, force: true }) };
}

export interface ObsidianInstance {
	page: Page;
	/** The vault root; the project is a folder in it (MyShop by default). */
	vault: string;
	errors: string[];
	carto(args: string[]): Promise<string>;
	git(args: string[]): Promise<string>;
	read(file: string): Promise<string>;
	close(): Promise<void>;
}

/**
 * Copies the MyShop test vault into a temporary folder with its own git and CARTO_HOME,
 * installs the plugin from obsidian/dist, and opens the vault in a separate Obsidian with
 * its own profile. The user's own Obsidian, vaults, and ~/.cartographer are not touched.
 */
/** The vault to copy, its project folder, and the folders that hold the clones of its repositories. */
export interface Source {
	vault: string;
	project: string;
	roots?: string[];
}

const MYSHOP: Source = { vault: path.join(REPO, "testdata", "vault"), project: "MyShop" };

export async function launchObsidian(bin: Carto, source: Source = MYSHOP): Promise<ObsidianInstance> {
	await stat(path.join(PLUGIN, "dist", "main.js")).catch(() => {
		throw new Error("dist has no build. Run npm run test:obsidian, which builds it first.");
	});
	const root = await mkdtemp(path.join(tmpdir(), "carto-e2e-"));
	const vault = path.join(root, "vault");
	const profile = path.join(root, "profile");
	const home = path.join(root, "carto-home");
	const pluginDir = path.join(vault, ".obsidian/plugins/cartographer");
	const env: NodeJS.ProcessEnv = { ...process.env, CARTO_HOME: home, GIT_AUTHOR_NAME: "Test", GIT_AUTHOR_EMAIL: "test@example.com", GIT_COMMITTER_NAME: "Test", GIT_COMMITTER_EMAIL: "test@example.com" };
	const project = path.join(vault, source.project);
	const carto = (args: string[]) => run(bin.path, [...args, "--project", project], { cwd: project, env });
	const git = (args: string[]) => run("git", args, { cwd: vault, env });

	let app: Running | undefined;
	try {
		await mkdir(home);
		await cp(source.vault, vault, { recursive: true });
		await mkdir(pluginDir, { recursive: true });
		for (const file of PLUGIN_FILES) await copyFile(path.join(PLUGIN, "dist", file), path.join(pluginDir, file));
		await writeFile(path.join(pluginDir, "data.json"), JSON.stringify({ binaryPath: bin.path }, null, 2));
		await git(["init", "-q", "-b", "main"]);
		await git(["add", "-A"]);
		await git(["commit", "-q", "--allow-empty", "-m", "test: the vault"]);
		await run(bin.path, ["setup", "--project", project, ...(source.roots ?? []).flatMap((r) => ["--root", r])], { cwd: vault, env });
		await carto(["render"]);

		await mkdir(profile);
		await copyAppUpdate(profile);
		await writeFile(path.join(profile, "obsidian.json"), JSON.stringify({ vaults: { cartotest00000001: { path: vault, ts: Date.now(), open: true } } }));
		app = await start(profile, env);
		await app.page.evaluate(async () => {
			const plugins = (window as any).app.plugins;
			if (plugins.plugins.cartographer?._loaded) return; // the vault turned it on already
			plugins.setEnable(true);
			await plugins.enablePluginAndSave("cartographer");
		});
		await pluginLoaded(app);
		await closeSettings(app.browser, app.page);
	} catch (error) {
		await app?.kill();
		await rm(root, { recursive: true, force: true });
		throw error;
	}
	const running = app;
	return {
		page: running.page,
		vault,
		errors: running.errors,
		carto,
		git,
		read: (file) => readFile(path.join(vault, file), "utf8"),
		async close() {
			await running.kill();
			await rm(root, { recursive: true, force: true, maxRetries: 10, retryDelay: 200 });
		},
	};
}

interface Running {
	browser: Browser;
	page: Page;
	errors: string[];
	kill(): Promise<void>;
}

const children = new Set<ChildProcess>();
process.once("exit", () => {
	for (const child of children) child.kill("SIGKILL");
});

async function start(profile: string, env: NodeJS.ProcessEnv): Promise<Running> {
	const port = await freePort();
	const log = process.env.OBSIDIAN_LOG;
	const child = spawn(OBSIDIAN, [`--user-data-dir=${profile}`, `--remote-debugging-port=${port}`, ...(log ? ["--enable-logging=stderr"] : [])], {
		stdio: log ? ["ignore", "ignore", openSync(log, "a")] : "ignore",
		env,
	});
	children.add(child);
	child.once("exit", () => children.delete(child));
	let browser: Browser | undefined;
	const kill = async () => {
		await browser?.close().catch(() => undefined);
		await stop(child);
	};
	try {
		browser = await connect(port);
		const page = await vaultPage(browser);
		if (!SHOW) await hide(page);
		const errors: string[] = [];
		page.on("console", (msg) => {
			if (msg.type() === "error") errors.push(msg.text());
		});
		page.on("pageerror", (error) => errors.push(error.stack ?? error.message));
		await page.waitForFunction(() => (window as any).app?.workspace?.layoutReady, undefined, { timeout: 30_000 });
		return { browser, page, errors, kill };
	} catch (error) {
		await kill();
		throw error;
	}
}

/** Hides Obsidian, which brings itself to the front, so the focus goes back to the user's app. */
async function hide(page: Page): Promise<void> {
	await page.waitForFunction(() => typeof (window as any).require === "function", undefined, { timeout: 30_000 });
	await page.evaluate(() => {
		const { remote } = (window as any).require("electron");
		remote.getCurrentWebContents().setBackgroundThrottling(false);
		remote.app.hide();
	});
}

async function pluginLoaded({ page, errors }: Running): Promise<void> {
	try {
		await page.waitForFunction(() => !!(window as any).app.plugins.plugins.cartographer?._loaded, undefined, { timeout: 10_000 });
	} catch {
		throw new Error(`The Cartographer plugin did not load in 10 seconds. Console errors:\n${errors.join("\n") || "none"}`);
	}
}

/** Turning on community plugins opens the settings a moment later, a modal or a window. */
async function closeSettings(browser: Browser, page: Page): Promise<void> {
	const others = () => browser.contexts().flatMap((c) => c.pages()).filter((p) => p !== page && p.url().startsWith("app://obsidian.md/"));
	const deadline = Date.now() + 3000;
	while (Date.now() < deadline) {
		if (others().length > 0 || (await page.$(".modal-container"))) break;
		await page.waitForTimeout(100);
	}
	for (const other of others()) await other.close().catch(() => undefined);
	for (let attempt = 0; attempt < 5 && (await page.$(".modal-container")); attempt++) {
		await page.keyboard.press("Escape");
		await page.waitForTimeout(200);
	}
}

/** Waits until check returns a value other than undefined or false, and returns it. */
export async function until<T>(what: string, check: () => Promise<T | undefined | false> | T | undefined | false, timeout = 15_000): Promise<T> {
	const deadline = Date.now() + timeout;
	for (;;) {
		const value = await check();
		if (value !== undefined && value !== false) return value;
		if (Date.now() > deadline) throw new Error(`Timed out after ${timeout} ms waiting for ${what}.`);
		await new Promise((resolve) => setTimeout(resolve, 100));
	}
}

/** A new profile would run the installer's older Obsidian; a copy of the installed update runs the user's version. */
async function copyAppUpdate(profile: string): Promise<void> {
	const updates = (await readdir(OBSIDIAN_DATA).catch(() => [] as string[])).filter((name) => /^obsidian-[\d.]+\.asar$/.test(name));
	const version = (name: string) => name.slice("obsidian-".length, -".asar".length).split(".").map(Number);
	const newest = updates
		.sort((a, b) => {
			const [x, y] = [version(a), version(b)];
			for (let i = 0; i < Math.max(x.length, y.length); i++) if ((x[i] ?? 0) !== (y[i] ?? 0)) return (x[i] ?? 0) - (y[i] ?? 0);
			return 0;
		})
		.at(-1);
	if (newest) await copyFile(path.join(OBSIDIAN_DATA, newest), path.join(profile, newest));
}

function run(program: string, args: string[], options: { cwd: string; env?: NodeJS.ProcessEnv }): Promise<string> {
	return new Promise((resolve, reject) => {
		execFile(program, args, { ...options, maxBuffer: 32 * 1024 * 1024 }, (error, stdout, stderr) => {
			if (error) reject(new Error(`${path.basename(program)} ${args.join(" ")} failed: ${stderr || error.message}`));
			else resolve(stdout);
		});
	});
}

async function connect(port: number): Promise<Browser> {
	const deadline = Date.now() + 30_000;
	for (;;) {
		try {
			return await chromium.connectOverCDP(`http://127.0.0.1:${port}`);
		} catch (error) {
			if (Date.now() > deadline) throw error;
			await new Promise((resolve) => setTimeout(resolve, 300));
		}
	}
}

async function vaultPage(browser: Browser): Promise<Page> {
	const deadline = Date.now() + 30_000;
	while (Date.now() < deadline) {
		for (const context of browser.contexts()) {
			for (const page of context.pages()) if (page.url().startsWith("app://obsidian.md/index.html")) return page;
		}
		await new Promise((resolve) => setTimeout(resolve, 300));
	}
	throw new Error("Obsidian did not open the vault window.");
}

function freePort(): Promise<number> {
	return new Promise((resolve, reject) => {
		const server = createServer();
		server.listen(0, "127.0.0.1", () => {
			const { port } = server.address() as { port: number };
			server.close(() => resolve(port));
		});
		server.on("error", reject);
	});
}

async function stop(child: ChildProcess): Promise<void> {
	if (child.exitCode !== null || child.signalCode !== null) return;
	const exited = new Promise((resolve) => child.once("exit", resolve));
	child.kill("SIGKILL");
	await exited;
}
