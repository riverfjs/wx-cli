---
name: wx-schedule
description: Manage scheduled tasks on the wx-cli serve cron scheduler. Use when user wants to set up recurring signal scans, periodic notifications, or any timed command execution tied to their WeChat profile.
---

# wx-schedule

## Goal

Add, list, delete, and toggle scheduled tasks on the wx-cli serve built-in cron scheduler. Each task runs with a scoped PUSH_KEY so it can only push to its own profile.

## Hard Constraints

- Always ensure wx serve is running before managing schedules.
- Always use `bash ~/.claude/skills/wx-schedule/scripts/schedule.sh` to manage tasks. Never call the HTTP endpoint directly.
- Always use standard 5-field cron format: `min hour day month weekday` (CST/Beijing time).
- Always verify the profile alias exists before adding tasks.
- Always check JSON output for `"ok":true` to confirm success.

## Workflow

### Add a scheduled task

```bash
bash ~/.claude/skills/wx-schedule/scripts/schedule.sh add \
  --profile e94921ef5bc5 \
  --cron "0 5 * * 1-5" \
  --cmd "python3 ~/.claude/skills/signal-monitor/scripts/scan.py AAPL.US --notify push"
```

### List tasks

```bash
bash ~/.claude/skills/wx-schedule/scripts/schedule.sh list --profile e94921ef5bc5
bash ~/.claude/skills/wx-schedule/scripts/schedule.sh list
```

### Delete a task

```bash
bash ~/.claude/skills/wx-schedule/scripts/schedule.sh del --id s1a2b3c4
```

### Enable / disable

```bash
bash ~/.claude/skills/wx-schedule/scripts/schedule.sh on --id s1a2b3c4
bash ~/.claude/skills/wx-schedule/scripts/schedule.sh off --id s1a2b3c4
```

## Cron Expression (Beijing time)

```
min hour day month weekday
0 5 * * 1-5       Mon-Fri 05:00
*/30 * * * *       Every 30 minutes
0 8,17 * * *       Daily 08:00 and 17:00
```

## Output

```json
{"ok":true,"id":"s1a2b3c4"}
{"ok":true,"schedules":[...]}
{"ok":false,"error":"..."}
```

## Common Patterns

### Signal monitor daily scan

```bash
bash ~/.claude/skills/wx-schedule/scripts/schedule.sh add \
  --profile e94921ef5bc5 \
  --cron "0 5 * * 1-5" \
  --cmd "python3 ~/.claude/skills/signal-monitor/scripts/scan.py AAPL.US SNDK.US --notify push"
```

## When NOT to use this skill

- User wants to run a one-off signal scan (use signal-monitor directly).
- User wants to send an immediate message (use wx-push).
- wx serve is not running.
