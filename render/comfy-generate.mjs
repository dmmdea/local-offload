// comfy-generate.mjs — local TEXT-TO-IMAGE runner: the single entrypoint the local-offload
// `generate_image` MCP tool shells out to. It wraps the proven, UNMODIFIED comfy-render.mjs
// (SDXL/RealVisXL via the ComfyUI HTTP API) with the lifecycle that the bare image renderer
// deliberately omits: single-slot GPU lock, free llama-swap first, start ComfyUI on-demand if
// it's down, free ComfyUI after (zero-always-warm). Mirrors comfy-video.mjs. Dependency-free.
//
// Usage:
//   node render/comfy-generate.mjs <out.png> "<prompt>" \
//        [--negative "..."] [--width 1024] [--height 1024] [--steps 30] [--seed N] \
//        [--ckpt name.safetensors] [--api http://127.0.0.1:8188] [--no-lock] [--keep-comfy]
import { existsSync } from "node:fs";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";
import { tmpdir } from "node:os";
import { spawn } from "node:child_process";
import { acquireGpuLock, freeLlamaSwap, freeComfy } from "./gpu-lock.mjs";

const __dirname = dirname(fileURLToPath(import.meta.url));
const argv = process.argv.slice(2);
const pos = []; const flags = {};
for (let i = 0; i < argv.length; i++) {
  if (argv[i].startsWith("--")) {
    const k = argv[i].slice(2);
    if (["no-lock", "keep-comfy"].includes(k)) flags[k] = true;
    else { flags[k] = argv[i + 1]; i++; }
  } else pos.push(argv[i]);
}
const out = pos[0], prompt = pos[1];
const API = flags.api || process.env.COMFY_API || "http://127.0.0.1:8188";
const COMFY_DIR = process.env.COMFY_DIR || "C:/ComfyUI";
// ComfyUI deps live in its venv, not the system python. Auto-detect; override via COMFY_PY.
const COMFY_PY = process.env.COMFY_PY
  || [".venv/Scripts/python.exe", "venv/Scripts/python.exe", "python_embeded/python.exe"]
       .map((p) => join(COMFY_DIR, p)).find((p) => existsSync(p))
  || "python";
if (!out || !prompt) {
  console.error('usage: node comfy-generate.mjs <out.png> "<prompt>" [--negative ...] [--width N] [--height N] [--steps N] [--seed N] [--ckpt name]');
  process.exit(2);
}

async function comfyUp() { try { const r = await fetch(API + "/system_stats"); return r.ok; } catch { return false; } }

async function ensureComfy() {
  if (await comfyUp()) return null; // already running — don't manage it
  const reserve = String(flags["reserve-vram"] || "1.0");
  const child = spawn(COMFY_PY, ["main.py", "--disable-smart-memory", "--cache-none", "--reserve-vram", reserve], { cwd: COMFY_DIR, stdio: "ignore", detached: false });
  for (let i = 0; i < 120; i++) { await new Promise((r) => setTimeout(r, 2000)); if (await comfyUp()) return child; }
  try { child.kill(); } catch {}
  throw new Error("ComfyUI did not become ready on " + API + " after ~4min");
}

// Delegate the actual render to the proven, unmodified comfy-render.mjs (ComfyUI is up now).
// comfy-render reads seed/width/height as flags OR positionals, so flags alone suffice.
function runRender() {
  const args = [join(__dirname, "comfy-render.mjs"), out, prompt, "--api", API];
  for (const k of ["negative", "width", "height", "steps", "seed", "ckpt", "vae", "cfg", "sampler", "scheduler"]) {
    if (flags[k] != null) args.push("--" + k, String(flags[k]));
  }
  return new Promise((resolve, reject) => {
    const c = spawn("node", args, { stdio: "inherit" });
    c.on("exit", (code) => (code === 0 ? resolve() : reject(new Error("comfy-render exited " + code))));
    c.on("error", reject);
  });
}

async function main() {
  const lockPath = process.env.GPU_LOCK || join(tmpdir(), "local-offload-gpu.lock");
  const lock = flags["no-lock"] ? { release() {} } : await acquireGpuLock({ lockPath });
  if (!lock) throw new Error("GPU is busy (another gen job holds the lock); try again later or --no-lock");
  let comfyChild = null;
  // Single teardown path (zero-always-warm): drop ComfyUI's VRAM, kill a ComfyUI we spawned,
  // release the GPU lock. Guarded so the finally and a signal can't double-run it.
  let cleaning = false;
  const cleanup = async () => {
    if (cleaning) return; cleaning = true;
    try { await freeComfy(); } catch {}
    if (comfyChild && !flags["keep-comfy"]) { try { comfyChild.kill(); } catch {} }
    try { lock.release(); } catch {}
  };
  // On a GRACEFUL interrupt (CLI Ctrl-C / a non-forced parent kill) run the same teardown so
  // the lock + VRAM are released instead of leaked. A forced SIGKILL/TerminateProcess still
  // bypasses this — the Go wrapper's process-tree kill + defer /free is the backstop for that.
  for (const sig of ["SIGINT", "SIGTERM", "SIGBREAK"]) {
    process.on(sig, async () => { await cleanup(); process.exit(130); });
  }
  try {
    await freeLlamaSwap();      // give the render the whole 8GB (GPU models only; CPU mem-stack stays warm)
    comfyChild = await ensureComfy();
    await runRender();
  } finally {
    await cleanup();
  }
}
main().catch((e) => { console.error("IMAGE GEN FAILED:", e.message); process.exit(1); });
