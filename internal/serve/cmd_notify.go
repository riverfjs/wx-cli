package serve

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func notifyDir() string {
	return filepath.Join(os.Getenv("HOME"), ".wx-cli", "notify")
}

func handleNotify(w http.ResponseWriter, msg *wxMessage, profile string, parts []string) {
	dir := notifyDir()
	flag := filepath.Join(dir, profile)

	if len(parts) < 2 {
		status := "关闭"
		if _, err := os.Stat(flag); err == nil {
			status = "开启"
		}
		replyPassive(w, msg, "Tool 执行通知: "+status+"\n用法: 通知 on/off")
		return
	}

	action := strings.ToLower(strings.TrimSpace(parts[1]))
	switch action {
	case "on", "开":
		os.MkdirAll(dir, 0755)
		os.WriteFile(flag, []byte("1"), 0644)
		replyPassive(w, msg, "Tool 执行通知已开启")
	case "off", "关":
		os.Remove(flag)
		replyPassive(w, msg, "Tool 执行通知已关闭")
	default:
		replyPassive(w, msg, "用法: 通知 on/off")
	}
}
