package serve

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func serveDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".wx-cli", "serve")
}

func serveProfilesPath() string {
	return filepath.Join(serveDir(), "profiles")
}

func saveServeProfile(profile string) {
	path := serveProfilesPath()
	os.MkdirAll(filepath.Dir(path), 0755)
	for _, p := range loadServeProfiles() {
		if p == profile {
			return
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString(profile + "\n")
}

func loadServeProfiles() []string {
	data, err := os.ReadFile(serveProfilesPath())
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func isServeProfile(profile string) bool {
	for _, p := range loadServeProfiles() {
		if p == profile {
			return true
		}
	}
	return false
}

// ── template persistence ──

func templatesPath() string {
	return filepath.Join(serveDir(), "templates.json")
}

func loadTemplates() map[string]string {
	data, err := os.ReadFile(templatesPath())
	if err != nil {
		return make(map[string]string)
	}
	var m map[string]string
	json.Unmarshal(data, &m)
	if m == nil {
		return make(map[string]string)
	}
	return m
}

func saveTemplates(m map[string]string) {
	os.MkdirAll(filepath.Dir(templatesPath()), 0755)
	data, _ := json.MarshalIndent(m, "", "  ")
	os.WriteFile(templatesPath(), data, 0600)
}

func getTemplate(name string) string {
	return loadTemplates()[name]
}

func setTemplate(name, templateID string) {
	m := loadTemplates()
	m[name] = templateID
	saveTemplates(m)
}

func delTemplate(name string) bool {
	m := loadTemplates()
	if _, ok := m[name]; !ok {
		return false
	}
	delete(m, name)
	saveTemplates(m)
	return true
}

func listTemplates() map[string]string {
	return loadTemplates()
}

func botScript() string {
	exe, _ := os.Executable()
	return filepath.Join(filepath.Dir(filepath.Dir(exe)), "shell", "bot.sh")
}

func startBot(profile string) error {
	script := botScript()
	if _, err := os.Stat(script); err != nil {
		return fmt.Errorf("bot.sh not found: %s", script)
	}
	cmd := exec.Command("bash", script, "start", profile)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func stopBot(profile string) error {
	cmd := exec.Command("bash", botScript(), "stop", profile)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func restartBot(profile string) error {
	stopBot(profile)
	return startBot(profile)
}
