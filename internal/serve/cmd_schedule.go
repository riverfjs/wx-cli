package serve

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func handleScheduleCmd(w http.ResponseWriter, msg *wxMessage, cfg *Config, profile string, isRoot bool, content string) {
	parts := strings.Fields(content)
	if len(parts) < 2 {
		replyPassive(w, msg, "用法:\n定时 add <cron> <命令>\n定时 list\n定时 del <id>\n定时 on/off <id>\n定时 log <id>")
		return
	}

	action := strings.ToLower(parts[1])
	switch action {
	case "add":
		if len(parts) < 4 {
			replyPassive(w, msg, "用法: 定时 add <cron表达式> <命令>\n例: 定时 add \"0 5 * * 1-5\" python3 scan.py AAPL.US --notify push")
			return
		}
		cronExpr := parts[2]
		command := strings.Join(parts[3:], " ")
		sch, err := sched.Add(profile, cronExpr, command)
		if err != nil {
			replyPassive(w, msg, "添加失败: "+err.Error())
			return
		}
		replyPassive(w, msg, fmt.Sprintf("定时任务已添加\nID: %s\nCron: %s\n命令: %s", sch.ID, sch.Cron, truncate(sch.Command, 40)))

	case "list":
		targetProfile := profile
		if isRoot && len(parts) >= 3 {
			if parts[2] == "all" {
				targetProfile = ""
			} else {
				targetProfile = parts[2]
			}
		}
		schedules := sched.List(targetProfile)
		if len(schedules) == 0 {
			replyPassive(w, msg, "无定时任务")
			return
		}
		var lines []string
		for _, s := range schedules {
			status := "✓"
			if !s.Enabled {
				status = "✗"
			}
			lines = append(lines, fmt.Sprintf("[%s] %s %s %s\n  %s", s.ID, status, s.Cron, shortID(s.Profile), truncate(s.Command, 40)))
		}
		replyPassive(w, msg, strings.Join(lines, "\n\n"))

	case "del":
		if len(parts) < 3 {
			replyPassive(w, msg, "用法: 定时 del <id>")
			return
		}
		id := parts[2]
		sch, ok := sched.Get(id)
		if !ok {
			replyPassive(w, msg, "任务不存在: "+id)
			return
		}
		if sch.Profile != profile && !isRoot {
			replyPassive(w, msg, "无权限")
			return
		}
		sched.Del(id)
		replyPassive(w, msg, "已删除: "+id)

	case "on":
		if len(parts) < 3 {
			replyPassive(w, msg, "用法: 定时 on <id>")
			return
		}
		id := parts[2]
		sch, ok := sched.Get(id)
		if !ok {
			replyPassive(w, msg, "任务不存在: "+id)
			return
		}
		if sch.Profile != profile && !isRoot {
			replyPassive(w, msg, "无权限")
			return
		}
		sched.SetEnabled(id, true)
		replyPassive(w, msg, "已启用: "+id)

	case "off":
		if len(parts) < 3 {
			replyPassive(w, msg, "用法: 定时 off <id>")
			return
		}
		id := parts[2]
		sch, ok := sched.Get(id)
		if !ok {
			replyPassive(w, msg, "任务不存在: "+id)
			return
		}
		if sch.Profile != profile && !isRoot {
			replyPassive(w, msg, "无权限")
			return
		}
		sched.SetEnabled(id, false)
		replyPassive(w, msg, "已禁用: "+id)

	case "log":
		if len(parts) < 3 {
			replyPassive(w, msg, "用法: 定时 log <id>")
			return
		}
		id := parts[2]
		sch, ok := sched.Get(id)
		if !ok {
			replyPassive(w, msg, "任务不存在: "+id)
			return
		}
		if sch.Profile != profile && !isRoot {
			replyPassive(w, msg, "无权限")
			return
		}
		logPath := filepath.Join(stateDir(), "logs", "schedule_"+id+".log")
		data, err := os.ReadFile(logPath)
		if err != nil {
			replyPassive(w, msg, "无执行日志")
			return
		}
		lines := strings.Split(string(data), "\n")
		start := 0
		if len(lines) > 20 {
			start = len(lines) - 20
		}
		replyPassive(w, msg, strings.Join(lines[start:], "\n"))

	default:
		replyPassive(w, msg, "用法:\n定时 add <cron> <命令>\n定时 list\n定时 del <id>\n定时 on/off <id>\n定时 log <id>")
	}
}

func handleScheduleAPI(w http.ResponseWriter, r *http.Request, cfg *Config) {
	secret := r.FormValue("secret")
	if secret != cfg.WxToken {
		http.Error(w, "invalid secret", http.StatusForbidden)
		return
	}

	action := r.FormValue("action")
	switch action {
	case "add":
		profile := r.FormValue("profile")
		cronExpr := r.FormValue("cron")
		command := r.FormValue("command")
		if profile == "" || cronExpr == "" || command == "" {
			http.Error(w, "missing profile, cron, or command", http.StatusBadRequest)
			return
		}
		sch, err := sched.Add(profile, cronExpr, command)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		data, _ := json.Marshal(map[string]interface{}{"ok": true, "id": sch.ID})
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)

	case "list":
		profile := r.FormValue("profile")
		schedules := sched.List(profile)
		data, _ := json.Marshal(map[string]interface{}{"ok": true, "schedules": schedules})
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)

	case "del":
		id := r.FormValue("id")
		if sched.Del(id) {
			fmt.Fprint(w, `{"ok":true}`)
		} else {
			http.Error(w, `{"ok":false,"error":"not found"}`, http.StatusNotFound)
		}

	case "on", "off":
		id := r.FormValue("id")
		if sched.SetEnabled(id, action == "on") {
			fmt.Fprint(w, `{"ok":true}`)
		} else {
			http.Error(w, `{"ok":false,"error":"not found"}`, http.StatusNotFound)
		}

	default:
		http.Error(w, "action: add/list/del/on/off", http.StatusBadRequest)
	}
}
