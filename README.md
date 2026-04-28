# wx-cli

WeChat personal account bot CLI, built on Tencent's official [iLink Bot API](https://github.com/Tencent/openclaw-weixin).

## Features

- QR code login (no app ID/secret needed)
- Send & receive text messages
- Send images, files, videos (AES-128-ECB encrypted CDN upload)
- Interactive REPL with contact management
- Daemon mode: `monitor` (JSON lines) + `send` (non-interactive) for bot integration
- Claude auto-reply bot with per-user history, rate limit guard, and 10s thinking timeout
- 5.9MB standalone Go binary

## Install as Claude Code skill

```bash
git clone <repo-url> ~/.claude/skills/wx-cli
cd ~/.claude/skills/wx-cli
bash shell/build.sh        # compile bin/wx
bin/wx login               # QR scan to authenticate
bash shell/wx-bot.sh       # start daemon
```

SKILL.md is included — Claude Code will auto-discover it after clone + build.

## Build

```bash
bash shell/build.sh
```

Binary output: `bin/wx`

## Usage

### Login

```bash
bin/wx login
```

Scan the QR code with WeChat. Token saved to `~/.wx-cli/accounts/`.

### Interactive mode

```bash
bin/wx
```

| Command | Action |
|---------|--------|
| *(text)* | Reply to active contact |
| `/contacts` | List contacts |
| `/use <index>` | Switch active contact |
| `/name <name>` | Set display name |
| `/image <path>` | Send image |
| `/file <path>` | Send file |
| `/video <path>` | Send video |
| `/status` | Connection info |
| `/login` | Re-authenticate |
| `/quit` | Exit |

### Daemon mode

Monitor incoming messages as JSON lines:

```bash
bin/wx monitor
# {"from_user_id":"o9cq...","context_token":"AAR...","type":"text","text":"你好","time":"00:15:03"}
```

Send non-interactively:

```bash
bin/wx send --to USER_ID --ctx CONTEXT_TOKEN --text "hello"
bin/wx send --to USER_ID --ctx CONTEXT_TOKEN --image /path/to/pic.png
bin/wx send --to USER_ID --ctx CONTEXT_TOKEN --file /path/to/doc.pdf
```

### Bot (Claude auto-reply)

```bash
bash shell/wx-bot.sh
```

Runs `wx monitor` → pipes messages to `claude --print` → replies via `wx send`.

**Built-in commands** (handled instantly, no Claude):

| Command | Response |
|---------|----------|
| `/ping` | `pong` (liveness check) |
| `/help` | List available commands |
| `/usage` | Claude rate limit status |

**Behavior:**
- Only text and voice transcription are processed; other types get a fallback reply.
- Per-user 1-round conversation history (`~/.wx-cli/history/{user_id}.json`), injected as context for continuity.
- System prompt (`prompts/system_role.md`) injected via `--append-system-prompt`, with `{to_user_id}` and `{context_token}` substituted per message.
- If Claude takes longer than 10s, a "Thinking..." message is sent first as feedback.
- Rate limit guard: rejects messages when Claude usage exceeds 85%.

## Architecture

```
main.go              Entry point
SKILL.md             Claude Code skill definition
api/                 Protocol types + HTTP client
cdn/                 AES-128-ECB encrypt + CDN upload
auth/                QR login + token persistence
send/                Send text/image/file/video
monitor/             Long-poll getupdates loop
cli/                 Interactive REPL
daemon/              monitor (JSON stdout) + send (non-interactive)
shell/
  build.sh           Build script → bin/wx
  wx-bot.sh          Claude auto-reply daemon
prompts/
  system_role.md     System prompt template for Claude
bin/                 Build output (gitignored)
reference/
  openclaw-weixin/   Official Tencent source (git submodule, protocol reference)
```

## Protocol notes

- API base: `https://ilinkai.weixin.qq.com`
- Success response: `{}` (no `ret` field). Error: `{"ret": -2, "errmsg": "..."}`
- Send wrapper: `{"msg": {WeixinMessage}}`, `from_user_id` must be `""`
- Media `aes_key`: `base64(utf8_bytes_of_hex_key)` — not `base64(raw_bytes)`
- Token valid ~24h, re-login required after expiry

## License

MIT
