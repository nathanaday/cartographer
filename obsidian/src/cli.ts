import { execFile, spawn } from "child_process";
import { existsSync, mkdirSync, openSync } from "fs";
import { homedir } from "os";
import { join } from "path";

export class CartoError extends Error {}

/** The binary to run: the setting, else the one `make install` puts in ~/.cartographer/bin. */
export function findBinary(setting: string): string | null {
	const candidates = [setting.trim(), join(process.env.CARTO_HOME || join(homedir(), ".cartographer"), "bin", "carto")];
	return candidates.find((c) => c !== "" && existsSync(c)) ?? null;
}

// Obsidian started from the Dock gets a short PATH; carto runs git, gh, and claude.
function childEnv(): NodeJS.ProcessEnv {
	const extra = [join(homedir(), ".local/bin"), "/opt/homebrew/bin", "/usr/local/bin", "/usr/bin", "/bin"];
	return { ...process.env, PATH: [process.env.PATH ?? "", ...extra].filter((p) => p !== "").join(":") };
}

function message(stderr: string, fallback: string): string {
	const line = stderr.trim().split("\n").pop() ?? "";
	return line.replace(/^carto: /, "") || fallback;
}

/** Runs carto in the project folder and returns its output. */
export function run(bin: string | null, folder: string, args: string[]): Promise<string> {
	if (!bin) return Promise.reject(new CartoError("the carto binary is not installed; run make install in the cartographer repository, or set its path in the settings"));
	return new Promise((resolve, reject) => {
		execFile(bin, [...args, "--project", folder], { cwd: folder, env: childEnv(), maxBuffer: 32 * 1024 * 1024, timeout: 2 * 60 * 1000 }, (err, stdout, stderr) => {
			if (!err) return resolve(stdout);
			if ((err as NodeJS.ErrnoException).code === "ENOENT") return reject(new CartoError(`no carto binary at ${bin}`));
			reject(new CartoError(message(String(stderr), err.message)));
		});
	});
}

/** Runs carto with --json and parses its output. */
export async function runJSON<T>(bin: string | null, folder: string, args: string[]): Promise<T> {
	const out = await run(bin, folder, [...args, "--json"]);
	try {
		return JSON.parse(out) as T;
	} catch {
		throw new CartoError(`carto ${args[0]} did not print JSON`);
	}
}

/**
 * Starts the runner in the background, detached from Obsidian, with its output in
 * .carto/runner.log of the vault. It ends by itself when only work for the user is left.
 */
export function startRunner(bin: string | null, folder: string, vaultRoot: string): void {
	if (!bin) throw new CartoError("the carto binary is not installed");
	const dir = join(vaultRoot, ".carto");
	mkdirSync(dir, { recursive: true });
	const log = openSync(join(dir, "runner.log"), "a");
	const child = spawn(bin, ["run", "--project", folder], { cwd: folder, env: childEnv(), detached: true, stdio: ["ignore", log, log] });
	child.unref();
}
