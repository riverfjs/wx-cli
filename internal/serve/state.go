package serve

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func stateDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".wx-cli")
}

func pidDir() string  { return filepath.Join(stateDir(), "pids") }
func histDir() string { return filepath.Join(stateDir(), "history") }

type ProcessInfo struct {
	Name string
	PID  int
	Alive bool
}

func checkProcess(name string) ProcessInfo {
	data, err := os.ReadFile(filepath.Join(pidDir(), name+".pid"))
	if err != nil {
		return ProcessInfo{Name: name}
	}
	var pid int
	fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &pid)
	alive := pid > 0 && syscall.Kill(pid, 0) == nil
	return ProcessInfo{Name: name, PID: pid, Alive: alive}
}

type BotInfo struct {
	Profile string
	PID     int
}

func listActiveBots() []BotInfo {
	entries, _ := os.ReadDir(pidDir())
	var bots []BotInfo
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".pid") {
			continue
		}
		if name == "serve.pid" || name == "ngrok.pid" {
			continue
		}
		profile := strings.TrimSuffix(name, ".pid")
		p := checkProcess(profile)
		if p.Alive {
			bots = append(bots, BotInfo{Profile: profile, PID: p.PID})
		}
	}
	return bots
}

type ChatRecord struct {
	Profile  string
	UserID   string
	Question string
	Answer   string
	ModTime  time.Time
}

func latestChat(profile string) *ChatRecord {
	profDir := filepath.Join(histDir(), profile)
	entries, _ := os.ReadDir(profDir)

	var latest os.DirEntry
	var latestTime time.Time
	for _, e := range entries {
		if info, err := e.Info(); err == nil {
			if latest == nil || info.ModTime().After(latestTime) {
				latest = e
				latestTime = info.ModTime()
			}
		}
	}
	if latest == nil {
		return nil
	}

	data, _ := os.ReadFile(filepath.Join(profDir, latest.Name()))
	var hist struct {
		Q string `json:"q"`
		A string `json:"a"`
	}
	json.Unmarshal(data, &hist)

	return &ChatRecord{
		Profile:  profile,
		UserID:   strings.TrimSuffix(latest.Name(), ".json"),
		Question: hist.Q,
		Answer:   hist.A,
		ModTime:  latestTime,
	}
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

func shortID(s string) string {
	if len(s) > 8 {
		return ".." + s[len(s)-8:]
	}
	return s
}
