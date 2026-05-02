package main

import (
	"fmt"
	"os"
	"strconv"

	"wx-cli/internal/api"
	"wx-cli/internal/auth"
	"wx-cli/internal/cli"
	"wx-cli/internal/msg"
	"wx-cli/internal/serve"
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

func requireProfile(profile string) {
	if profile == "" {
		fmt.Fprintln(os.Stderr, "Profile required. Usage: wx --profile NAME <command>")
		os.Exit(1)
	}
}

func requireCred(profile string) *api.Credential {
	requireProfile(profile)
	cred := auth.LoadCredential(profile)
	if cred == nil {
		fmt.Fprintf(os.Stderr, "Profile '%s' not found. Run: wx login --profile %s\n", profile, profile)
		os.Exit(1)
	}
	return cred
}

func parseSendArgs(args []string) (to, ctx, text, image, file, video string) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--to":
			if i+1 < len(args) {
				to = args[i+1]; i++
			}
		case "--ctx":
			if i+1 < len(args) {
				ctx = args[i+1]; i++
			}
		case "--text":
			if i+1 < len(args) {
				text = args[i+1]; i++
			}
		case "--image":
			if i+1 < len(args) {
				image = args[i+1]; i++
			}
		case "--file":
			if i+1 < len(args) {
				file = args[i+1]; i++
			}
		case "--video":
			if i+1 < len(args) {
				video = args[i+1]; i++
			}
		}
	}
	return
}

func envDefault(val, envKey string) string {
	if val != "" {
		return val
	}
	return os.Getenv(envKey)
}

func parseServeArgs(args []string) (port int, wxToken, wxAppID, wxSecret, wxRoot string) {
	port = 8080
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--port":
			if i+1 < len(args) {
				port, _ = strconv.Atoi(args[i+1]); i++
			}
		case "--wx-token":
			if i+1 < len(args) {
				wxToken = args[i+1]; i++
			}
		case "--wx-appid":
			if i+1 < len(args) {
				wxAppID = args[i+1]; i++
			}
		case "--wx-secret":
			if i+1 < len(args) {
				wxSecret = args[i+1]; i++
			}
		case "--wx-root":
			if i+1 < len(args) {
				wxRoot = args[i+1]; i++
			}
		}
	}
	return
}

func parseTypingArgs(args []string) (to, ctx, status string) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--to":
			if i+1 < len(args) {
				to = args[i+1]; i++
			}
		case "--ctx":
			if i+1 < len(args) {
				ctx = args[i+1]; i++
			}
		case "--status":
			if i+1 < len(args) {
				status = args[i+1]; i++
			}
		}
	}
	return
}

func main() {
	profile, args := parseGlobal(os.Args[1:])

	if len(args) == 0 {
		requireProfile(profile)
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
		requireProfile(profile)
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
		msg.Monitor(requireCred(profile), profile)

	case "send":
		cred := requireCred(profile)
		to, ctx, text, image, file, video := parseSendArgs(args[1:])

		if to == "" || ctx == "" {
			fmt.Fprintln(os.Stderr, "Usage: wx send --to USER_ID --ctx CONTEXT_TOKEN --text MSG")
			fmt.Fprintln(os.Stderr, "       wx send --to USER_ID --ctx CONTEXT_TOKEN --image PATH")
			fmt.Fprintln(os.Stderr, "       wx send --to USER_ID --ctx CONTEXT_TOKEN --file PATH")
			os.Exit(1)
		}

		var err error
		switch {
		case text != "":
			err = msg.Text(cred, to, ctx, text)
		case image != "":
			err = msg.Image(cred, to, ctx, image)
		case file != "":
			err = msg.File(cred, to, ctx, file)
		case video != "":
			err = msg.Video(cred, to, ctx, video)
		default:
			fmt.Fprintln(os.Stderr, "Specify --text, --image, --file, or --video")
			os.Exit(1)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Send failed: %s\n", err)
			os.Exit(1)
		}

	case "typing":
		cred := requireCred(profile)
		to, ctx, status := parseTypingArgs(args[1:])
		if to == "" || ctx == "" {
			fmt.Fprintln(os.Stderr, "Usage: wx --profile NAME typing --to USER_ID --ctx TOKEN --status start|stop")
			os.Exit(1)
		}
		ticket, err := api.GetConfig(cred, to, ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "GetConfig failed: %s\n", err)
			os.Exit(1)
		}
		s := 1
		if status == "stop" {
			s = 2
		}
		if err := api.SendTyping(cred, to, ticket, s); err != nil {
			fmt.Fprintf(os.Stderr, "SendTyping failed: %s\n", err)
			os.Exit(1)
		}

	case "serve":
		port, wxToken, wxAppID, wxSecret, wxRoot := parseServeArgs(args[1:])
		wxToken = envDefault(wxToken, "WX_TOKEN")
		wxAppID = envDefault(wxAppID, "WX_APPID")
		wxSecret = envDefault(wxSecret, "WX_SECRET")
		wxRoot = envDefault(wxRoot, "WX_ROOT")
		if wxToken == "" {
			fmt.Fprintln(os.Stderr, "WeChat token required. Use --wx-token or WX_TOKEN env var")
			os.Exit(1)
		}
		serve.Run(serve.Config{
			Port:       port,
			WxToken:    wxToken,
			WxAppID:    wxAppID,
			WxSecret:   wxSecret,
			RootOpenID: wxRoot,
		})

	case "help", "--help", "-h":
		fmt.Printf(`%swx-cli%s — WeChat iLink Bot CLI

%sUsage:%s
  wx --profile NAME login                       QR code login
  wx accounts                                   List saved profiles
  wx --profile NAME monitor                     Daemon: JSON lines per message
  wx --profile NAME send --to ID --ctx TOKEN --text MSG
  wx --profile NAME typing --to ID --ctx TOKEN --status start|stop
  wx --profile NAME                             Interactive REPL
  wx serve --wx-token TOKEN [--port 8080]       WeChat webhook server
`, cB, cW, cB, cW)

	default:
		requireProfile(profile)
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
