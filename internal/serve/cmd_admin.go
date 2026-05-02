package serve

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

func handleService(w http.ResponseWriter, msg *wxMessage) {
	fmtProc := func(p ProcessInfo) string {
		if !p.Alive {
			return p.Name + ": 未运行"
		}
		return fmt.Sprintf("%s: 运行中 (PID %d)", p.Name, p.PID)
	}

	lines := []string{
		fmtProc(checkProcess("serve")),
		fmtProc(checkProcess("ngrok")),
		fmt.Sprintf("管理 profile 数: %d", len(listManagedAliases())),
		fmt.Sprintf("注册模板数: %d", len(listTemplates())),
		fmt.Sprintf("定时任务数: %d", len(sched.List(""))),
	}
	replyPassive(w, msg, strings.Join(lines, "\n"))
}

func handleActiveBots(w http.ResponseWriter, msg *wxMessage) {
	bots := listActiveBots()
	if len(bots) == 0 {
		replyPassive(w, msg, "无活跃 Bot")
		return
	}
	var lines []string
	for _, b := range bots {
		lines = append(lines, fmt.Sprintf("%s (PID %d)", shortID(b.Profile), b.PID))
	}
	replyPassive(w, msg, fmt.Sprintf("活跃 Bot (%d):\n%s", len(lines), strings.Join(lines, "\n")))
}

func handleChatHistory(w http.ResponseWriter, msg *wxMessage, target string) {
	var profiles []string
	if target != "" {
		profiles = []string{target}
	} else {
		for _, b := range listActiveBots() {
			profiles = append(profiles, b.Profile)
		}
	}
	if len(profiles) == 0 {
		replyPassive(w, msg, "无活跃 Bot 或未指定 profile")
		return
	}

	var lines []string
	for _, profile := range profiles {
		chat := latestChat(profile)
		label := shortID(profile)
		if chat == nil {
			lines = append(lines, fmt.Sprintf("[%s] 无聊天记录", label))
			continue
		}
		ago := time.Since(chat.ModTime).Truncate(time.Minute)
		lines = append(lines, fmt.Sprintf("[%s] %s (%s前)\nQ: %s\nA: %s",
			label, shortID(chat.UserID), ago,
			truncate(chat.Question, 50), truncate(chat.Answer, 80)))
	}
	replyPassive(w, msg, strings.Join(lines, "\n\n"))
}

func handleRestartBots(w http.ResponseWriter, msg *wxMessage) {
	bots := listActiveBots()
	if len(bots) == 0 {
		replyPassive(w, msg, "无活跃 Bot")
		return
	}
	var lines []string
	for _, b := range bots {
		if err := restartBot(b.Profile); err != nil {
			lines = append(lines, fmt.Sprintf("%s: 重启失败 %s", shortID(b.Profile), err))
		} else {
			lines = append(lines, fmt.Sprintf("%s: 已重启", shortID(b.Profile)))
		}
	}
	replyPassive(w, msg, fmt.Sprintf("重启 %d 个 Bot:\n%s", len(bots), strings.Join(lines, "\n")))
}

func handleStopBots(w http.ResponseWriter, msg *wxMessage) {
	bots := listActiveBots()
	if len(bots) == 0 {
		replyPassive(w, msg, "无活跃 Bot")
		return
	}
	var lines []string
	for _, b := range bots {
		if err := stopBot(b.Profile); err != nil {
			lines = append(lines, fmt.Sprintf("%s: 停止失败 %s", shortID(b.Profile), err))
		} else {
			lines = append(lines, fmt.Sprintf("%s: 已停止", shortID(b.Profile)))
		}
	}
	replyPassive(w, msg, fmt.Sprintf("停止 %d 个 Bot:\n%s", len(bots), strings.Join(lines, "\n")))
}

func handleBroadcast(w http.ResponseWriter, msg *wxMessage, cfg *Config, text string) {
	aliases := listManagedAliases()
	count := 0
	for _, a := range aliases {
		if oid, ok := resolveAlias(a); ok {
			sendAsync(cfg, oid, text)
			count++
		}
	}
	replyPassive(w, msg, fmt.Sprintf("已广播给 %d 个用户", count))
}

func handleTemplate(w http.ResponseWriter, msg *wxMessage, content string) {
	parts := strings.Fields(content)
	if len(parts) < 2 {
		replyPassive(w, msg, "用法:\n模板 add <name> <id>\n模板 list\n模板 del <name>")
		return
	}

	action := strings.ToLower(parts[1])
	switch action {
	case "add":
		if len(parts) < 4 {
			replyPassive(w, msg, "用法: 模板 add <name> <template_id>")
			return
		}
		setTemplate(parts[2], parts[3])
		replyPassive(w, msg, fmt.Sprintf("模板已注册: %s", parts[2]))
	case "list":
		tpls := listTemplates()
		if len(tpls) == 0 {
			replyPassive(w, msg, "无已注册模板")
			return
		}
		var lines []string
		for name, id := range tpls {
			lines = append(lines, fmt.Sprintf("%s: %s", name, id[:8]+"..."))
		}
		replyPassive(w, msg, strings.Join(lines, "\n"))
	case "del":
		if len(parts) < 3 {
			replyPassive(w, msg, "用法: 模板 del <name>")
			return
		}
		if delTemplate(parts[2]) {
			replyPassive(w, msg, fmt.Sprintf("模板已删除: %s", parts[2]))
		} else {
			replyPassive(w, msg, fmt.Sprintf("模板不存在: %s", parts[2]))
		}
	default:
		replyPassive(w, msg, "用法:\n模板 add <name> <id>\n模板 list\n模板 del <name>")
	}
}
