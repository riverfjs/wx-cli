package serve

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

func serveDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".wx-cli", "serve")
}

// ── OpenID → hash alias mapping ──

var (
	openIDMap   = make(map[string]string) // alias → openID
	openIDMapMu sync.RWMutex
)

func hashOpenID(openID string) string {
	h := sha256.Sum256([]byte(openID))
	return hex.EncodeToString(h[:])[:12]
}

func registerOpenID(openID string) string {
	alias := hashOpenID(openID)
	openIDMapMu.Lock()
	openIDMap[alias] = openID
	openIDMapMu.Unlock()
	saveMapping()
	return alias
}

func resolveAlias(alias string) (openID string, ok bool) {
	openIDMapMu.RLock()
	defer openIDMapMu.RUnlock()
	openID, ok = openIDMap[alias]
	return
}

func listManagedAliases() []string {
	openIDMapMu.RLock()
	defer openIDMapMu.RUnlock()
	var out []string
	for alias := range openIDMap {
		out = append(out, alias)
	}
	return out
}

func mappingPath() string {
	return filepath.Join(serveDir(), "mapping.json")
}

func loadMapping() {
	data, err := os.ReadFile(mappingPath())
	if err != nil {
		return
	}
	openIDMapMu.Lock()
	defer openIDMapMu.Unlock()
	json.Unmarshal(data, &openIDMap)
	log.Printf("[wx-serve] loaded %d profile mappings", len(openIDMap))
}

func saveMapping() {
	openIDMapMu.RLock()
	data, _ := json.MarshalIndent(openIDMap, "", "  ")
	openIDMapMu.RUnlock()
	os.MkdirAll(serveDir(), 0755)
	os.WriteFile(mappingPath(), data, 0600)
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
	os.MkdirAll(serveDir(), 0755)
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

// ── bot process management ──

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
