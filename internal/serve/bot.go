package serve

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func serveProfilesPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".wx-cli", "serve_profiles")
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
