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

func parseGlobal(args []string) (profile string, rest []string) {
	for i := 0; i < len(args); i++ {
		if args[i] == "--profile" && i+1 < len(args) {
			profile = args[i+1]
			i++
		} else {
			rest = append(rest, args[i])
		}
	}
	return
}

func requireCred(profile string) *api.Credential {
	cred := auth.LoadCredential(profile)
	if cred == nil {
		if profile != "" {
			fmt.Fprintf(os.Stderr, "Profile '%s' not found. Run: wx login --profile %s\n", profile, profile)
		} else {
			fmt.Fprintln(os.Stderr, "Not logged in. Run: wx login")
		}
		os.Exit(1)
	}
	return cred
}

func main() {
	profile, args := parseGlobal(os.Args[1:])

	if len(args) == 0 {
		cred := auth.LoadCredential(profile)
		if cred == nil {
			fmt.Println("No saved account. Starting login...\n")
			var err error
			cred, err = auth.Login(profile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Login failed: %s\n", err)
				os.Exit(1)
			}
		}
		cli.Interactive(cred, profile)
		return
	}

	switch args[0] {
	case "login", "auth":
		if _, err := auth.Login(profile); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}

	case "accounts":
		accs := auth.ListAccounts()
		if len(accs) == 0 {
			fmt.Println("No accounts. Run: wx login --profile NAME")
			return
		}
		for _, a := range accs {
			fmt.Printf("  %s%-16s%s  bot_id=%s  login=%s\n", cB, a.Profile, cW, a.Cred.BotID, a.Cred.LoginTime)
		}

	case "monitor":
		daemon.Monitor(requireCred(profile), profile)

	case "send":
		daemon.Send(requireCred(profile), args[1:])

	case "help", "--help", "-h":
		fmt.Printf(`%swx-cli%s — WeChat iLink Bot CLI

%sUsage:%s
  wx login [--profile NAME]                     QR code login
  wx accounts                                   List saved profiles
  wx monitor [--profile NAME]                   Daemon: JSON lines per message
  wx send [--profile NAME] --to ID --ctx TOKEN --text MSG
  wx [--profile NAME]                           Interactive REPL
`, cB, cW, cB, cW)

	default:
		cred := auth.LoadCredential(profile)
		if cred == nil {
			fmt.Println("No saved account. Starting login...\n")
			var err error
			cred, err = auth.Login(profile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Login failed: %s\n", err)
				os.Exit(1)
			}
		}
		cli.Interactive(cred, profile)
	}
}
