# wx-cli

WeChat personal account bot CLI, built on Tencent's official [iLink Bot API](https://github.com/Tencent/openclaw-weixin).

## Features

- QR code login with named profiles (no app ID/secret needed)
- Send & receive text messages
- Send images, files, videos (AES-128-ECB encrypted CDN upload)
- Interactive REPL with contact management
- Daemon mode: `monitor` (JSON lines) + `send` (non-interactive) for bot integration
- Claude auto-reply bot with per-user conversation history and thinking feedback
- Multi-profile support: run multiple accounts simultaneously
- 5.9MB standalone Go binary

## Install as Claude Code skill

```bash
git clone <repo-url> ~/.claude/skills/wx-cli
cd ~/.claude/skills/wx-cli
bash shell/build.sh                    # compile bin/wx
bin/wx login --profile mybot           # QR scan, save as "mybot"
bash shell/wx-bot.sh mybot             # start Claude auto-reply daemon
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
bin/wx login --profile work       # save as "work"
bin/wx login --profile personal   # another account as "personal"
bin/wx login                      # no profile, saved by bot_id
```

Scan the QR code with WeChat. Token saved to `~/.wx-cli/accounts/{profile}.json`.

### List accounts

```bash
bin/wx accounts
```

### Interactive mode

```bash
bin/wx --profile work     # or just bin/wx (uses most recent login)
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
bin/wx monitor --profile work
# {"from_user_id":"o9cq...","context_token":"AAR...","type":"text","text":"你好","time":"00:15:03"}
```

Send non-interactively:

```bash
bin/wx send --profile work --to USER_ID --ctx CONTEXT_TOKEN --text "hello"
bin/wx send --profile work --to USER_ID --ctx CONTEXT_TOKEN --image /path/to/pic.png
bin/wx send --profile work --to USER_ID --ctx CONTEXT_TOKEN --file /path/to/doc.pdf
```

### Bot (Claude auto-reply)

```bash
bash shell/wx-bot.sh work
```

Flow: `wx monitor` → parse JSON → dispatch to `shell/handlers/reply.sh` → `claude --print` → `wx send`.

**Built-in commands** (instant, no Claude):

| Command | Response |
|---------|----------|
| `/ping` | `pong` (liveness check) |
| `/help` | List available commands |
| `/usage` | Claude rate limit status |

**Behavior:**
- Text and voice transcription only; other types get a fallback reply.
- Per-profile per-user 1-round conversation history (`~/.wx-cli/history/{profile}/{user_id}.json`) injected as context.
- System prompt (`prompts/system_role.md`) injected with `{profile}`, `{to_user_id}`, `{context_token}` substituted per message. Claude can send images/files via `wx send` in the Bash tool.
- 5s thinking timeout: sends "Thinking..." feedback if Claude hasn't replied yet.
- Rate limit guard: rejects at 85% Claude usage.
- Stale sync cursor auto-reset: 3 consecutive session-paused errors clears the cursor.

## Architecture

```
main.go              Entry point
SKILL.md             Claude Code skill definition
api/                 Protocol types + HTTP client
cdn/                 AES-128-ECB encrypt + CDN upload
auth/                QR login + token persistence + multi-profile
send/                Send text/image/file/video
monitor/             Long-poll getupdates loop
cli/                 Interactive REPL
daemon/              monitor (JSON stdout) + send (non-interactive)
shell/
  build.sh           Build script → bin/wx
  wx-bot.sh          Claude auto-reply daemon (monitor + dispatch)
  handlers/
    reply.sh         Per-message handler (claude --print → wx send)
prompts/
  system_role.md     System prompt template for Claude
bin/                 Build output (gitignored)
reference/
  openclaw-weixin/   Official Tencent source (git submodule, protocol reference)
```

## Data layout

```
~/.wx-cli/
  accounts/{profile}.json       Credentials per profile
  sync_buf_{profile}            Long-poll cursor per profile
  history/{profile}/{uid}.json  1-round conversation history per user per profile
```

## Protocol notes

- API base: `https://ilinkai.weixin.qq.com`
- Success response: `{}` (no `ret` field). Error: `{"ret": -2, "errmsg": "..."}`
- Send wrapper: `{"msg": {WeixinMessage}}`, `from_user_id` must be `""`
- Media `aes_key`: `base64(utf8_bytes_of_hex_key)` — not `base64(raw_bytes)`
- Token valid ~24h, re-login required after expiry

## License

MIT
