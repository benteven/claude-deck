# claude-deck

A terminal app for people who run many Claude Code sessions at once.

Running sessions sit in a list on the left with the live session on the right. Sessions you close stay findable: newest first, searchable, with their last known state shown in gray. Resume one with Enter.

claude-deck is not made by or affiliated with Anthropic.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/benteven/claude-deck/main/install.sh | sh
```

This installs a release binary to `~/.local/bin`. Set `CLAUDE_DECK_BIN_DIR` to use another folder, or `CLAUDE_DECK_VERSION` to pin a release. You need tmux.

With Homebrew:

```sh
brew install benteven/tap/claude-deck
```

This also installs tmux. You need [Claude Code](https://docs.claude.com/en/docs/claude-code) on your PATH. `gh` is optional; with it, rows show the state of linked pull requests.

From source (Go 1.27 or later):

```sh
go install github.com/benteven/claude-deck@latest
```

macOS and Linux. On Linux, `d` needs `gio` or `trash-cli` to move files to the Trash.

## Use

Run `claude-deck` in a plain terminal, not inside tmux.

```
Active 4  Suspended 23
  1 api refactor                 ┃ > fix the review comments
  ━━━━━━━━━━━━ 72% ctx           ┃
  #4631 draft  2 artifacts       ┃ ● Edit src/routes.ts
    ↳ Review routes (tests)      ┃ ● Bash go test ./...
  2 docs cleanup ● waiting       ┃
  ━━━━━━━━━━━━ 31% ctx           ┃ the live Claude Code session
  Elsewhere                      ┃
  billing spike                  ┃
```

Each row shows the context used, open pull requests, linked artifacts, running subagents, and a mark when the session waits for you.

### Keys

From anywhere:

| Key | Does |
| --- | --- |
| `ctrl-b` | Focus the list |
| `ctrl-j` / `ctrl-k` | Next or previous session |
| `ctrl-b 1` ... `ctrl-b 9` | Go to session 1 to 9 |
| `ctrl-b <` / `ctrl-b >` | Narrower or wider list |
| `ctrl-b ctrl-b`, `ctrl-b ctrl-j`, `ctrl-b ctrl-k` | Send that key to Claude Code |

In the list:

| Key | Does |
| --- | --- |
| arrows, `j` / `k` | Move; the right side follows |
| Enter | Open the session, or resume a suspended one |
| Esc | Back to the session you came from |
| `1` ... `9` | Go to that session |
| Tab | Switch between Active and Suspended |
| `/` | Search names, prompts, folders, branches and PR numbers |
| `n` | New session in a recent folder |
| `s` | Suspend: rename it, then stop it |
| `r` | Rename |
| `R` | Restart the session in place, for example after Claude Code updates |
| `t` | Take over a session running in another terminal |
| `o` | Open the row's pull requests |
| `d` | Move a suspended session to the Trash |
| `q` | Close the app; sessions keep running |
| `Q` | Stop all sessions and quit |

Run `claude-deck` again to come back to running sessions. After a restart, sessions that were active show in gray and the app offers to resume them.

## How it works

claude-deck runs its own tmux server (socket `claude-deck`), so your own tmux setup is not touched. The list is a Bubble Tea program in the left pane. Each Claude Code session is a real process in its own tmux pane, swapped into the right side when you select it.

Session data comes from Claude Code's own files in `~/.claude`, read only. The one exception is `d`, which moves a transcript to the Trash. Sessions started by the app get hooks through `claude --settings`, which mark when a session waits for you; your global Claude Code settings are not changed.

App state lives in `~/.config/claude-deck`. Set `"prefix"` in `config.json` to use another key than `ctrl-b`, `"claude_args"` to pass extra flags to every session, and `"list_width"` (24 to 100, default 36) for the list width. Drag the border or press `ctrl-b <` and `ctrl-b >` to change it and keep the new width.

## License

MIT
