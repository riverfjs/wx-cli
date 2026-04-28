# wx-cli

WeChat personal account bot CLI, built on Tencent's official [iLink Bot API](https://github.com/Tencent/openclaw-weixin).

## Features

- QR code login (no app ID/secret needed)
- Send & receive text messages
- Send images, files, videos (AES-128-ECB encrypted CDN upload)
- Interactive REPL with contact management
- Daemon mode: `monitor` (JSON lines) + `send` (non-interactive) for bot integration
- 5.9MB standalone Go binary

## Build

```bash
go build -ldflags="-s -w" -o wx .
```

## Usage

### Login

```bash
./wx login
```

Scan the QR code with WeChat. Token saved to `~/.wx-cli/accounts/`.

### Interactive mode

```bash
./wx
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
./wx monitor
# {"from_user_id":"o9cq...","context_token":"AAR...","type":"text","text":"你好","time":"00:15:03"}
```

Send non-interactively:

```bash
./wx send --to USER_ID --ctx CONTEXT_TOKEN --text "hello"
./wx send --to USER_ID --ctx CONTEXT_TOKEN --image /path/to/pic.png
./wx send --to USER_ID --ctx CONTEXT_TOKEN --file /path/to/doc.pdf
```

### Bot (Claude auto-reply)

```bash
bash wx-bot.sh
```

Runs `wx monitor` → pipes messages to `claude --print` → replies via `wx send`.

## Architecture

```
main.go          Entry point
api/             Protocol types + HTTP client
cdn/             AES-128-ECB encrypt + CDN upload
auth/            QR login + token persistence
send/            Send text/image/file/video
monitor/         Long-poll getupdates loop
cli/             Interactive REPL
daemon/          monitor (JSON stdout) + send (non-interactive)
wx-bot.sh        Claude auto-reply daemon script
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
