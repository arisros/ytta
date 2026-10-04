// ytta: reports this Pi's sessions to ytta, the tmux plugin that shows
// which agents need you. Written by `ytta install --pi` and removed by
// `ytta uninstall --pi`; edits are lost.
import { spawn } from "node:child_process"
import { createHash, randomUUID } from "node:crypto"

const YTTA = "__YTTA__"

const fallback = randomUUID()

// One `ytta hook` at a time, in the order the events happened: ytta's
// state machine needs them in order, and Pi never waits for it.
let queue = Promise.resolve()

function run(payload) {
  queue = queue.then(
    () =>
      new Promise((done) => {
        try {
          const p = spawn(YTTA, ["hook", "--agent", "pi"], { stdio: ["pipe", "ignore", "ignore"] })
          p.on("error", done)
          p.on("close", done)
          p.stdin.on("error", () => {})
          p.stdin.end(JSON.stringify(payload))
        } catch {
          done()
        }
      }),
  )
}

// The session's id, or its file hashed so that no path leaves Pi.
function session(ctx) {
  const sm = ctx?.sessionManager
  const id = sm?.getSessionId?.() ?? sm?.getSessionFile?.() ?? fallback
  return createHash("sha256").update(String(id)).digest("hex").slice(0, 16)
}

// A Pi run without its terminal interface is a script's or another agent's,
// and says nothing about the pane it happens to share.
function report(event, ctx, extra) {
  if (ctx?.mode ? ctx.mode !== "tui" : !ctx?.hasUI) return
  run({ hook_event_name: event, session_id: session(ctx), ...extra })
}

export default function (pi) {
  pi.on("session_start", async (_event, ctx) => report("SessionStart", ctx, { source: "startup" }))
  pi.on("session_shutdown", async (_event, ctx) => report("SessionEnd", ctx))
  pi.on("agent_start", async (_event, ctx) => report("UserPromptSubmit", ctx))
  pi.on("tool_execution_start", async (event, ctx) => report("PreToolUse", ctx, { tool_name: String(event?.toolName ?? "") }))
  pi.on("tool_execution_end", async (event, ctx) => report("PostToolUse", ctx, { tool_name: String(event?.toolName ?? "") }))
  pi.on("agent_end", async (_event, ctx) => report("Stop", ctx))
}
