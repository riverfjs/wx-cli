package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mdp/qrterminal/v3"
	"wx-cli/api"
)

func stateDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".wx-cli")
}

func accountsDir() string {
	return filepath.Join(stateDir(), "accounts")
}

func SaveCredential(cred *api.Credential, profile string) string {
	dir := accountsDir()
	os.MkdirAll(dir, 0755)
	name := profile
	if name == "" {
		name = cred.BotID
	}
	if name == "" {
		name = "default"
	}
	path := filepath.Join(dir, name+".json")
	data, _ := json.MarshalIndent(cred, "", "  ")
	os.WriteFile(path, data, 0600)
	return path
}

func LoadCredential(profile string) *api.Credential {
	dir := accountsDir()
	if profile != "" {
		path := filepath.Join(dir, profile+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var c api.Credential
		json.Unmarshal(data, &c)
		return &c
	}
	entries, _ := os.ReadDir(dir)
	var best *api.Credential
	var bestTime string
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, _ := os.ReadFile(filepath.Join(dir, e.Name()))
		var c api.Credential
		json.Unmarshal(data, &c)
		if c.LoginTime > bestTime {
			bestTime = c.LoginTime
			cc := c
			best = &cc
		}
	}
	return best
}

type AccountInfo struct {
	Profile string
	Cred    api.Credential
}

func ListAccounts() []AccountInfo {
	dir := accountsDir()
	entries, _ := os.ReadDir(dir)
	var out []AccountInfo
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, _ := os.ReadFile(filepath.Join(dir, e.Name()))
		var c api.Credential
		json.Unmarshal(data, &c)
		out = append(out, AccountInfo{
			Profile: strings.TrimSuffix(e.Name(), ".json"),
			Cred:    c,
		})
	}
	return out
}

func SyncBufPath(profile string) string {
	if profile != "" {
		return filepath.Join(stateDir(), "sync_buf_"+profile)
	}
	return filepath.Join(stateDir(), "sync_buf")
}

func LoadSyncBuf(profile string) string {
	data, err := os.ReadFile(SyncBufPath(profile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func SaveSyncBuf(profile string, buf string) {
	os.MkdirAll(stateDir(), 0755)
	os.WriteFile(SyncBufPath(profile), []byte(buf), 0644)
}

func Login(profile string) (*api.Credential, error) {
	fmt.Println("Requesting QR code...")
	qrcode, qrURL, err := api.GetQrCode()
	if err != nil {
		return nil, err
	}

	fmt.Println("\nScan with WeChat:\n")
	qrterminal.GenerateHalfBlock(qrURL, qrterminal.L, os.Stdout)
	fmt.Printf("\nOr open: %s\n\nWaiting for scan...\n", qrURL)

	lastStatus := ""
	for {
		time.Sleep(2 * time.Second)
		status, cred, err := api.GetQrStatus(qrcode)
		if err != nil {
			continue
		}
		if status == "expired" {
			return nil, fmt.Errorf("QR code expired")
		}
		if status == "scaned" && lastStatus != "scaned" {
			fmt.Println("Scanned! Confirm on phone...")
		}
		if status == "scaned_but_redirect" && lastStatus != "scaned_but_redirect" {
			fmt.Println("Redirecting...")
		}
		if status == "confirmed" && cred != nil {
			path := SaveCredential(cred, profile)
			fmt.Printf("\nLogin successful! Saved to %s\n", path)
			return cred, nil
		}
		lastStatus = status
	}
}
