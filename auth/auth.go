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

func SaveCredential(cred *api.Credential) string {
	dir := accountsDir()
	os.MkdirAll(dir, 0755)
	id := cred.BotID
	if id == "" {
		id = "default"
	}
	path := filepath.Join(dir, id+".json")
	data, _ := json.MarshalIndent(cred, "", "  ")
	os.WriteFile(path, data, 0600)
	return path
}

func LoadCredential(id string) *api.Credential {
	dir := accountsDir()
	if id != "" {
		path := filepath.Join(dir, id+".json")
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

func ListCredentials() []api.Credential {
	dir := accountsDir()
	entries, _ := os.ReadDir(dir)
	var out []api.Credential
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, _ := os.ReadFile(filepath.Join(dir, e.Name()))
		var c api.Credential
		json.Unmarshal(data, &c)
		out = append(out, c)
	}
	return out
}

func LoadSyncBuf() string {
	data, err := os.ReadFile(filepath.Join(stateDir(), "sync_buf"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func SaveSyncBuf(buf string) {
	os.MkdirAll(stateDir(), 0755)
	os.WriteFile(filepath.Join(stateDir(), "sync_buf"), []byte(buf), 0644)
}

func Login() (*api.Credential, error) {
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
			path := SaveCredential(cred)
			fmt.Printf("\nLogin successful! Saved to %s\n", path)
			return cred, nil
		}
		lastStatus = status
	}
}
