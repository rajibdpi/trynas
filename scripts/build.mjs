#!/usr/bin/env node
import { spawnSync } from "node:child_process";
import { mkdirSync, rmSync, existsSync, readFileSync, writeFileSync, readdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { createHash } from "node:crypto";

const project = dirname(dirname(fileURLToPath(import.meta.url)));
const frontend = join(project, "frontend"), dist = join(project, "dist");
const available = ["linux/amd64", "linux/arm64", "windows/amd64", "windows/arm64", "darwin/amd64", "darwin/arm64"];
const hostOS = { win32: "windows", darwin: "darwin", linux: "linux" }[process.platform];
const hostArch = { x64: "amd64", arm64: "arm64" }[process.arch];
const args = process.argv.slice(2);
if (args.includes("--help")) { console.log("node scripts/build.mjs [--all | --target linux/amd64,windows/amd64,darwin/arm64]\nDefault: current operating system and architecture. Outputs binaries and SHA256SUMS to dist/."); process.exit(0) }
let targets = [`${hostOS}/${hostArch}`];
if (args.includes("--all")) targets = available;
else if (args.includes("--target")) { const value = args[args.indexOf("--target") + 1]; if (!value) throw new Error("--target requires os/architecture"); targets = value.split(",") }
if (args.some(arg => !(["--all", "--target"].includes(arg) || targets.join(",") === arg))) throw new Error("Unknown build argument; use --help");
for (const target of targets) if (!available.includes(target)) throw new Error(`Unsupported target: ${target}`);
function run(command, params, cwd, extraEnv = {}) { const windowsPnpm = process.platform === "win32" && command === "pnpm"; const result = spawnSync(windowsPnpm ? "cmd.exe" : command, windowsPnpm ? ["/d", "/s", "/c", [command, ...params].join(" ")] : params, { cwd, env: { ...process.env, ...extraEnv }, stdio: "inherit", shell: false }); if (result.error) throw result.error; if (result.status !== 0) throw new Error(`${command} exited with ${result.status}`) }
if (!existsSync(join(frontend, "node_modules"))) run("pnpm", ["install", "--frozen-lockfile"], frontend);
// Never reuse a prior static export or the running dev server's build directory.
rmSync(join(frontend, ".next"), { recursive: true, force: true });
rmSync(join(frontend, "dist"), { recursive: true, force: true });
console.log("Building the current frontend from clean release output...");
run("pnpm", ["build"], frontend, { TRYNAS_EMBEDDED_BUILD: "true", NEXT_PUBLIC_EMBEDDED: "true", NEXT_PUBLIC_API_URL: "" });
const exported = join(frontend, "dist"); if (!existsSync(join(exported, "index.html"))) throw new Error("Static frontend export is missing");
mkdirSync(dist, { recursive: true });
const sums = [];
for (const target of targets) { const [os, arch] = target.split("/"); const name = `trynas-${os}-${arch}${os === "windows" ? ".exe" : ""}`; console.log(`Building ${target}...`); run("go", ["build", "-trimpath", "-ldflags=-s -w", "-o", join(dist, name), "."], project, { CGO_ENABLED: "0", GOOS: os, GOARCH: arch }); sums.push(`${createHash("sha256").update(readFileSync(join(dist, name))).digest("hex")}  ${name}`) }
// Remove the obsolete credentials artifact from earlier builds.
rmSync(join(dist, "admin-credentials.txt"), { force: true });
console.log("Fresh installations require ADMIN_PASSWORD at runtime; release binaries contain no default password.");
// Include every built artifact when building a subset into an existing dist folder.
for (const name of readdirSync(dist).filter(n => /^trynas-(linux|windows|darwin)-/.test(n))) { if (!sums.some(s => s.endsWith(`  ${name}`))) sums.push(`${createHash("sha256").update(readFileSync(join(dist, name))).digest("hex")}  ${name}`) }
writeFileSync(join(dist, "SHA256SUMS"), sums.sort().join("\n") + "\n");
console.log(`Updated frontend and backend embedded in: ${targets.map(target => `trynas-${target.replace("/", "-")}${target.startsWith("windows/") ? ".exe" : ""}`).join(", ")}`);
console.log(`Standalone builds ready in ${dist}. Restart the built executable to serve this release; building does not replace a running process.`);
console.log("From the project root, stop the development backend and run your executable in dist/ to use the existing .env, database and storage.");
