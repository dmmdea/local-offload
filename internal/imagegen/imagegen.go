// Package imagegen generates an image from a text prompt by shelling out to the repo's
// render/comfy-generate.mjs (Node), which wraps the proven comfy-render.mjs with the
// GPU-lock + ComfyUI start/stop lifecycle. The render runs on the LOCAL ComfyUI (SDXL/
// RealVisXL) — free, no cloud. Pure os/exec, no deps; mirrors internal/audioio's pattern.
package imagegen

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Generate runs `node <script> <out> <prompt> [--negative ..] [--width ..] ...` and
// returns out on success. node is the node executable ("" => "node"); script is the
// absolute path to comfy-generate.mjs; comfyDir is exported as COMFY_DIR for the script.
// params may carry negative (string) and width/height/steps/seed (int-ish). A non-zero
// exit, a timeout, or a missing/empty output file returns an error (the caller defers).
func Generate(ctx context.Context, node, script, comfyDir, out, prompt string, params map[string]any, timeout time.Duration) (string, error) {
	if node == "" {
		node = "node"
	}
	if script == "" {
		return "", fmt.Errorf("imagegen: no script configured")
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := []string{script, out, prompt}
	if n, ok := params["negative"].(string); ok && n != "" {
		args = append(args, "--negative", n)
	}
	for _, k := range []string{"width", "height", "steps", "seed"} {
		if v := asInt(params[k]); v > 0 {
			args = append(args, "--"+k, strconv.Itoa(v))
		}
	}
	cmd := exec.CommandContext(cctx, node, args...)
	cmd.Env = append(os.Environ(), "COMFY_DIR="+comfyDir)
	// On timeout/cancel, Go's default kill hits only the bare `node` process; on Windows that
	// ORPHANS the ComfyUI python grandchild (pinning ~8GB VRAM) and bypasses node's JS finally
	// (leaking the GPU lock). Kill the WHOLE process tree instead, with a short grace window.
	cmd.Cancel = func() error { return killTree(cmd.Process) }
	cmd.WaitDelay = 10 * time.Second
	// Belt-and-suspenders: however node ended (clean exit, error, or a timeout-kill that skipped
	// its finally), force-drop any ComfyUI VRAM so a render never leaves the GPU pinned
	// (zero-always-warm; protects the load-bearing CPU memory stack). Best-effort.
	defer freeComfyVRAM(comfyAPI())

	o, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("imagegen: comfy-generate failed: %w (%s)", err, tail(o, 400))
	}
	if fi, statErr := os.Stat(out); statErr != nil || fi.Size() == 0 {
		return "", fmt.Errorf("imagegen: no output image at %q (%s)", out, tail(o, 400))
	}
	return out, nil
}

// asInt coerces an any (int / int64 / float64) to int; 0 on miss.
func asInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

// killTree force-terminates p and ALL of its descendants. On Windows, killing the bare node
// process leaves the spawned ComfyUI python alive (no process-group semantics), so we taskkill
// the whole tree; elsewhere a direct kill is the best portable effort.
func killTree(p *os.Process) error {
	if p == nil {
		return nil
	}
	if runtime.GOOS == "windows" {
		_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(p.Pid)).Run()
		return nil
	}
	return p.Kill()
}

// comfyAPI is where ComfyUI listens (override via COMFY_API), matching comfy-generate.mjs.
func comfyAPI() string {
	if v := os.Getenv("COMFY_API"); v != "" {
		return v
	}
	return "http://127.0.0.1:8188"
}

// freeComfyVRAM asks ComfyUI to unload models + free VRAM (zero-always-warm). Best-effort:
// a 1s timeout and any error are ignored (ComfyUI may already be gone, or never ours to free).
func freeComfyVRAM(api string) {
	cl := &http.Client{Timeout: 1 * time.Second}
	req, err := http.NewRequest(http.MethodPost, api+"/free", strings.NewReader(`{"unload_models":true,"free_memory":true}`))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if resp, derr := cl.Do(req); derr == nil {
		_ = resp.Body.Close()
	}
}

// tail returns the last n bytes of b as a string (so a long ComfyUI stack trace is bounded).
func tail(b []byte, n int) string {
	if len(b) > n {
		b = b[len(b)-n:]
	}
	return string(b)
}
