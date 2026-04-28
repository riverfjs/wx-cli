package main

import (
	"fmt"
	"os"

	"wx-cli/api"
	"wx-cli/auth"
	"wx-cli/cli"
	"wx-cli/daemon"
)

const (
	cB = "\033[1m"
	cW = "\033[0m"
)

func requireCred() *api.Credential {
	cred := auth.LoadCredential("")
	if cred == nil {
		fmt.Fprintln(os.Stderr, "Not logged in. Run: wx login")
		os.Exit(1)
	}
	return cred
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		cred := auth.LoadCredential("")
		if cred == nil {
			fmt.Println("No saved account. Starting login...\n")
			var err error
			cred, err = auth.Login()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Login failed: %s\n", err)
				os.Exit(1)
			}
		}
		cli.Interactive(cred)
		return
	}

	switch args[0] {
	case "login", "auth":
		if _, err := auth.Login(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}

	case "accounts":
		creds := auth.ListCredentials()
		if len(creds) == 0 {
			fmt.Println("No accounts. Run: wx login")
			return
		}
		for _, c := range creds {
			fmt.Printf("  %s%s%s  baseurl=%s  login=%s\n", cB, c.BotID, cW, c.BaseURL, c.LoginTime)
		}

	case "monitor":
		daemon.Monitor(requireCred())

	case "send":
		daemon.Send(requireCred(), args[1:])

	case "help", "--help", "-h":
		fmt.Printf(`%swx-cli%s — WeChat iLink Bot CLI

%sUsage:%s
  wx login                                  QR code login
  wx accounts                               List saved accounts
  wx monitor                                Daemon: JSON lines per message
  wx send --to ID --ctx TOKEN --text MSG    Send text
  wx send --to ID --ctx TOKEN --image PATH  Send image
  wx send --to ID --ctx TOKEN --file PATH   Send file
  wx                                        Interactive REPL
`, cB, cW, cB, cW)

	default:
		var accountID string
		for i := 0; i < len(args); i++ {
			if args[i] == "--account" && i+1 < len(args) {
				accountID = args[i+1]
				i++
			}
		}
		cred := auth.LoadCredential(accountID)
		if cred == nil {
			fmt.Println("No saved account. Starting login...\n")
			var err error
			cred, err = auth.Login()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Login failed: %s\n", err)
				os.Exit(1)
			}
		}
		cli.Interactive(cred)
	}
}
