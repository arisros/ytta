# Roadmap

What ytta set out to do, what is done, and what it will not become. The order was set by one goal: someone who is not the author can install it, trust it and use it with the agent they already run.

## What decides adoption

| Question a new user asks | Answer today |
|---|---|
| Does it support my agent? | Claude Code, Codex, Gemini CLI, opencode, Copilot CLI, Droid, Qwen Code, Kilo Code CLI, Pi, Kimi Code CLI and Hermes Agent. Only Claude Code's behaviour is replayed from recorded sessions |
| Does it work on my tmux? | 3.2 and newer, which covers Ubuntu 22.04 and RHEL 9. CI runs 3.2a, 3.3a and current |
| Does it ever lie to me? | every state has a way out, endings no hook reports are read from the screen, agents that exited are cleared, and every state change is logged with its cause |
| Does it save me a trip to the pane? | the reason an agent waits, its screen in the popup, answering and prompting from there, a notify command |

## Rules every change keeps

| Rule | Enforced by |
|---|---|
| Nothing runs between events, and a hook makes at most one tmux call | `test/integration/perf_test.go`, `TestHotPathSkipsTmux` |
| A state is never guessed: the screen is trusted only for an agent's explicit markers | `TestClassify`, `TestCodexClassify` |
| Prompts and tool arguments are never kept, only the event and the tool name | `internal/hook`, the fixture leak scanner, `TestReasonAndEventLog` |
| ytta only types into a pane whose agent is still running | `TestSendAndInterruptReachOnlyALiveAgent` |
| A hook stays under 8 ms with no state change and 18 ms with one (p50, 120 panes) | `make perf` |
| Your panes, keys and layouts stay as they are | integration tests on private tmux servers |

## Done

| # | Change | What it added |
|---|---|---|
| 1 | Why an agent is waiting, and a log of state changes | `permission Bash`, `question`, `elicitation`, `input` or `dialog` beside a waiting agent; the source of every state (hook, screen, focus); `events.jsonl` and `ytta events`; fixture replay that asserts every transition |
| 2 | Agents that exited are cleared | each hook remembers the pane's foreground command, so liveness needs no list of process names; views sweep the dead; doctor checks keys, the sound player and stale agents; the machine is fuzzed |
| 3 | A command when an agent needs you | `@ytta-notify-command`, run by tmux beside the sound |
| 4 | tmux 3.2 | the suite passed on 3.2a once the popup's border style and title were dropped there, so 3.2 is the floor; a statusLine of your own can be wrapped |
| 5 | Codex | the agent boundary in `internal/agent`; Codex's hooks, its Interrupt event, its dialogs |
| 6 | Preview and answer | the selected agent's screen in the popup; send a prompt, answer a dialog, interrupt, mark seen; `ytta send`, `ytta interrupt` |
| 7 | Order, filter, context | longest wait first; `field:word` filters; the attention toggle; the git branch without a `git` process; consistent JSON |
| 8 | Gemini CLI and opencode | Gemini through its hooks, opencode through a plugin file run under test with node |
| 9 | Scripting | `ytta events --follow` and `ytta wait`, both asleep in `tmux wait-for` between events |
| 10 | Positioning | the tagline, the compatibility table and the agent guide in [CONTRIBUTING.md](../CONTRIBUTING.md) |
| 11 | Follow-ups | the selected agent's timeline and session age in the popup; copy and rename; `@ytta-popup-attention`; `started_at`; tokens and cost for opencode through `ytta usage`, which any agent can call |

## Left to do

| What | Why it is open |
|---|---|
| Recorded sessions for Codex, Gemini CLI, opencode, Copilot CLI, Droid, Qwen Code, Kilo Code CLI, Pi, Kimi Code CLI and Hermes Agent | their mappings were read from documentation and source. ytta's rule is that transitions come from recordings, and these do not meet it yet |
| What a denied approval looks like in Codex | no hook was found for it, and its screen has no end marker ytta has seen, so the agent may show as running until the next prompt |
| Whether Gemini CLI's hooks see `TMUX_PANE` | its environment redaction may hide it; without it ytta cannot tell which pane the agent is in |
| Claude Code fixtures for resume, compaction, a crash, a failed tool, an elicitation | they need real sessions to record |
| `sort:` terms in the filter | the order is fixed: urgency, then longest wait |
| Sending a prompt into a live agent | tested against a stand-in that echoes its input; the paste followed by Enter has not been watched in each real agent |

## What each agent reports

Read from each project's source and documentation on 2026-10-02.

| What ytta needs | Claude Code | Codex 0.150+ | Gemini CLI 0.62 | opencode 1.18 |
|---|---|---|---|---|
| Mechanism | hooks in `settings.json` | hooks in `~/.codex/hooks.json`, same shape | hooks in `~/.gemini/settings.json`, same shape | a plugin file in `~/.config/opencode/plugins/` |
| Event names | baseline | identical | different (`BeforeAgent`, `AfterTool`) | different (`session.idle`, `permission.asked`) |
| Session end | `SessionEnd` | `SessionEnd`, from 0.145 | `SessionEnd` | none on exit; the pane's command changing ends it |
| Waiting | `PermissionRequest`, `Notification` | `PermissionRequest`, from 0.124 | `Notification` of type `ToolPermission` | `permission.asked`, `question.asked` |
| A denied permission | no event, read from the screen | no event found | the next `BeforeModel` | `permission.replied` |
| Interrupt | no event, read from the screen | `Interrupt`, from 0.150 | unknown | `session.error`, then `session.idle` |
| Background work at the end of a turn | reported by `Stop` | the screen says `background terminal running` | unknown | not found |
| Process name tmux sees | its version number, or `claude` | `node` when installed from npm | not observed | `opencode`, or `node` from npm |
| Usage for another program | statusLine JSON | none outside its transcript file | not checked | tokens and cost per message, summed by the plugin and sent through `ytta usage` |

Agents added later are listed by row. Read from each project's documentation on 2026-10-04.

| Agent | Mechanism | Waiting | A denied permission | Interrupt | Session end |
|---|---|---|---|---|---|
| Copilot CLI | ytta's own file in `~/.copilot/hooks/`, registered under the PascalCase names so payloads carry Claude Code's fields | `notification` of type `permission_prompt` or `elicitation_dialog`; `permissionRequest` is not used, it fires before the rules decide whether to ask | no event found | not documented | `SessionEnd` |
| Droid | hooks in `~/.factory/settings.json`, Claude Code's shape and names | `Notification` of type `permission_prompt` or `elicitation_dialog` | no event found | a `Notification` in place of `Stop`, taken to be `idle_prompt` | `SessionEnd` |
| Qwen Code | hooks in `~/.qwen/settings.json`, Claude Code's shape and names | `PermissionRequest`, `Notification` of type `permission_prompt`; a question has no event yet | `PermissionDenied` only for its own classifier, not for the user's answer | no event found | `SessionEnd` |
| Kilo Code CLI | the opencode plugin in `~/.config/kilo/plugin/`, exported as `{ id, server }` | `permission.asked`; `question.asked` if Kilo publishes it as opencode does | `permission.replied` | `session.error`, then `session.idle` | none on exit; the pane's command changing ends it |
| Pi | an extension file in `~/.pi/agent/extensions/` | none: Pi has no permission prompts | does not apply | `agent_end`, if Pi sends it for an aborted turn | `session_shutdown` |
| Kimi Code CLI | `[[hooks]]` tables in `~/.kimi-code/config.toml` | `PermissionRequest` | `PermissionResult` | `Interrupt` | `SessionEnd` |
| Hermes Agent | shell hooks under `hooks:` in `~/.hermes/config.yaml`, run without a shell and after a consent prompt | `pre_approval_request` | `post_approval_response` | `on_session_end`, which is the end of every turn and says whether it was interrupted | `on_session_finalize` |

## Agents considered

Read from each project's documentation on 2026-10-04. An agent is added only when its own hooks or plugins report every state it has, because a blocked agent shown as running is the lie ytta exists to prevent.

| Agent | Decision | What is missing | What would change it |
|---|---|---|---|
| Cursor CLI | deferred | no permission hook, and its question tool skips the tool hooks | a permission or notification hook in the CLI |
| Goose | deferred | no permission, question or idle event | an approval event in its hooks |
| Kiro CLI | deferred | no permission or question trigger; a session end only on the opt-in V3 engine | an approval trigger, and V3 as the default |
| Amp | deferred | its plugin API has no permission, question or session end event | those events in the plugin API |
| Grok Build | deferred | `PermissionDenied` only after the fact | a permission request event, or a notification on the prompt |
| Auggie | deferred | no prompt or permission event | both |
| OpenHands CLI | deferred | no approval event, and hooks are configured per repository only | an approval event and a user level hooks file |
| Mistral Vibe | deferred | three events: before a tool, after a tool, the end of a turn | session, prompt and approval events |
| Devin CLI | deferred | the hooks file's place and the payload's event name were not found | published documentation of both |
| Continue CLI | no target of its own | `cn` reads the hooks in `~/.claude/settings.json` as well as its own, and its payload does not say which program sent it. With ytta installed for Claude Code it is already tracked, shown as Claude; a second set of hooks would fire every event twice | a field in its payload naming the caller, or a way to keep it from reading Claude Code's settings |
| Aider | declined | one command with no payload, run both when a turn ends and when it asks to confirm | hooks that tell the two apart |
| Crush | declined | `PreToolUse` only, so a running agent could never leave that state | the rest of its planned hooks |
| Cline CLI | declined | its plugins see no approval or question | an approval hook |

## tmux versions

| Distribution | tmux | Supported |
|---|---|---|
| Ubuntu 22.04, RHEL, Rocky and Alma 9 | 3.2a | yes, with a popup that has no border style or title |
| Debian 12, EL 10 | 3.3a | yes |
| Ubuntu 24.04 and 26.04, Debian 13, Fedora, Amazon Linux 2023, Alpine, Arch, Homebrew, nix | 3.4 and newer | yes |
| Ubuntu 20.04, Debian 11, EL 8 | 3.1c and older | no: `display-popup` and `run-shell -C` arrived in 3.2 |

## Considered and declined

| Idea | Why not |
|---|---|
| More states (blocked, failed, unknown) | no hook reports them, so they would be guesses. A reason on the existing states carries what is actually known |
| A confidence score per state | there is no honest source for the number. The source of a state says the same thing truthfully |
| Storing the command or question an agent is waiting on | it would put prompts and tool arguments on disk. The preview shows the real dialog live, with nothing kept |
| Changed file and test counts per agent | a `git` process per agent on every refresh |
| Kanban and switchable layouts | interface surface with no pull; tabs, borders, the sidebar and the popup already cover compact to detailed |
| A webhook system and built-in Slack or Telegram | the notify command covers all of them in one option |
| A socket or HTTP API | it needs a process that stays alive. The CLI with JSON output is the API |
| Usage bars for Codex | Codex writes them only to its transcript, which ytta does not read |
| A Homebrew tap, for now | brew would install only the binary. The plugin runs the copy in its own directory, which tpm already downloads with a checksum, so a tap adds a second copy and nothing a tmux user needs |

## Non-goals

- a terminal multiplexer, a PTY layer or a terminal emulator: tmux is the runtime
- a daemon, or any process that outlives the event or the command that started it
- reading transcript files, or calling a model provider's API
- remote machines and SSH session management
- a web dashboard
