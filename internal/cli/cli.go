package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"wx-cli/internal/api"
	"wx-cli/internal/auth"
	"wx-cli/internal/msg"
)

const (
	cG   = "\033[32m"
	cR   = "\033[31m"
	cY   = "\033[33m"
	cC   = "\033[36m"
	cB   = "\033[1m"
	cW   = "\033[0m"
	cDIM = "\033[2m"
)

type contact struct {
	userID       string
	contextToken string
	lastSeen     time.Time
	name         string
}

var (
	contacts      = make(map[string]*contact)
	mu            sync.Mutex
	activeContact string
	curProfile    string
)

func shortID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:4] + ".." + id[len(id)-4:]
}

func getActive() *contact {
	mu.Lock()
	defer mu.Unlock()
	if activeContact == "" {
		return nil
	}
	return contacts[activeContact]
}

func displayMsg(m *api.WeixinMessage) {
	if m.MessageType == 2 {
		return
	}
	from := m.FromUserID

	mu.Lock()
	c, ok := contacts[from]
	if m.ContextToken != "" {
		if !ok {
			c = &contact{userID: from, name: shortID(from)}
			contacts[from] = c
		}
		c.contextToken = m.ContextToken
		c.lastSeen = time.Now()
	}
	mu.Unlock()

	name := shortID(from)
	if c != nil {
		name = c.name
	}
	t := time.Now().Format("15:04:05")

	for _, item := range m.ItemList {
		switch item.Type {
		case 1:
			txt := ""
			if item.TextItem != nil {
				txt = item.TextItem.Text
			}
			fmt.Printf("\n%s[%s]%s %s%s%s%s: %s\n", cC, t, cW, cG, cB, name, cW, txt)
		case 2:
			fmt.Printf("\n%s[%s]%s %s%s%s%s: %s[Image]%s\n", cC, t, cW, cG, cB, name, cW, cY, cW)
		case 3:
			txt := ""
			if item.VoiceItem != nil && item.VoiceItem.Text != "" {
				txt = fmt.Sprintf(` "%s"`, item.VoiceItem.Text)
			}
			dur := 0
			if item.VoiceItem != nil {
				dur = item.VoiceItem.Playtime
			}
			fmt.Printf("\n%s[%s]%s %s%s%s%s: %s[Voice %dms]%s%s\n", cC, t, cW, cG, cB, name, cW, cY, dur, cW, txt)
		case 4:
			fn := ""
			if item.FileItem != nil {
				fn = item.FileItem.FileName
			}
			fmt.Printf("\n%s[%s]%s %s%s%s%s: %s[File]%s %s\n", cC, t, cW, cG, cB, name, cW, cY, cW, fn)
		case 5:
			fmt.Printf("\n%s[%s]%s %s%s%s%s: %s[Video]%s\n", cC, t, cW, cG, cB, name, cW, cY, cW)
		}
	}

	mu.Lock()
	if activeContact == "" {
		activeContact = from
		fmt.Printf("%s  (auto-selected as active contact)%s\n", cDIM, cW)
	}
	mu.Unlock()
}

func handleLine(line string, cred *api.Credential) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	if strings.HasPrefix(line, "/") {
		parts := strings.Fields(line)
		cmd := strings.ToLower(parts[0])
		arg := ""
		if len(parts) > 1 {
			arg = strings.Join(parts[1:], " ")
		}

		switch cmd {
		case "/help":
			fmt.Printf("\n%sCommands:%s\n", cB, cW)
			fmt.Println("  /contacts          List contacts")
			fmt.Println("  /use <id|index>    Switch active contact")
			fmt.Println("  /name <name>       Set display name")
			fmt.Println("  /image <path>      Send image")
			fmt.Println("  /file <path>       Send file")
			fmt.Println("  /video <path>      Send video")
			fmt.Println("  /status            Connection info")
			fmt.Println("  /login             Re-authenticate")
			fmt.Println("  /quit              Exit")
			fmt.Println()

		case "/contacts":
			mu.Lock()
			if len(contacts) == 0 {
				fmt.Printf("%sNo contacts yet.%s\n", cY, cW)
			} else {
				fmt.Printf("\n%sContacts:%s\n", cB, cW)
				i := 0
				for id, c := range contacts {
					active := ""
					if id == activeContact {
						active = fmt.Sprintf(" %s<< active%s", cG, cW)
					}
					ago := int(time.Since(c.lastSeen).Seconds())
					fmt.Printf("  %s%d%s %s%s%s %s(%s)%s %s%ds ago%s%s\n",
						cDIM, i, cW, cC, c.name, cW, cDIM, shortID(id), cW, cDIM, ago, cW, active)
					i++
				}
			}
			mu.Unlock()

		case "/use":
			if arg == "" {
				fmt.Printf("%sUsage: /use <index|name>%s\n", cR, cW)
				return
			}
			mu.Lock()
			if idx, err := strconv.Atoi(arg); err == nil {
				i := 0
				for id, c := range contacts {
					if i == idx {
						activeContact = id
						fmt.Printf("%sSwitched to %s%s\n", cG, c.name, cW)
						break
					}
					i++
				}
			} else {
				found := false
				for id, c := range contacts {
					if c.name == arg || id == arg || shortID(id) == arg {
						activeContact = id
						fmt.Printf("%sSwitched to %s%s\n", cG, c.name, cW)
						found = true
						break
					}
				}
				if !found {
					fmt.Printf("%sContact not found%s\n", cR, cW)
				}
			}
			mu.Unlock()

		case "/name":
			c := getActive()
			if c == nil {
				fmt.Printf("%sNo active contact%s\n", cR, cW)
				return
			}
			if arg == "" {
				fmt.Printf("%sUsage: /name <name>%s\n", cR, cW)
				return
			}
			mu.Lock()
			c.name = arg
			mu.Unlock()
			fmt.Printf("%sRenamed to %s%s\n", cG, arg, cW)

		case "/image", "/file", "/video":
			c := getActive()
			if c == nil {
				fmt.Printf("%sNo active contact%s\n", cR, cW)
				return
			}
			if arg == "" {
				fmt.Printf("%sUsage: %s <path>%s\n", cR, cmd, cW)
				return
			}
			path, _ := filepath.Abs(arg)
			if _, err := os.Stat(path); err != nil {
				fmt.Printf("%sFile not found: %s%s\n", cR, path, cW)
				return
			}
			fmt.Printf("%sUploading %s...%s\n", cDIM, path, cW)
			var err error
			switch cmd {
			case "/image":
				err = msg.Image(cred, activeContact, c.contextToken, path)
			case "/video":
				err = msg.Video(cred, activeContact, c.contextToken, path)
			default:
				err = msg.File(cred, activeContact, c.contextToken, path)
			}
			if err != nil {
				fmt.Printf("%sFailed: %s%s\n", cR, err, cW)
			} else {
				fmt.Printf("%sSent!%s\n", cG, cW)
			}

		case "/status":
			fmt.Printf("%sBot ID:%s %s\n", cB, cW, cred.BotID)
			fmt.Printf("%sBase URL:%s %s\n", cB, cW, cred.BaseURL)
			fmt.Printf("%sLogin:%s %s\n", cB, cW, cred.LoginTime)
			mu.Lock()
			fmt.Printf("%sContacts:%s %d\n", cB, cW, len(contacts))
			if c, ok := contacts[activeContact]; ok {
				fmt.Printf("%sActive:%s %s\n", cB, cW, c.name)
			}
			mu.Unlock()

		case "/login":
			newCred, err := auth.Login(curProfile)
			if err != nil {
				fmt.Printf("%sLogin failed: %s%s\n", cR, err, cW)
			} else {
				*cred = *newCred
				fmt.Printf("%sRe-login successful%s\n", cG, cW)
			}

		case "/quit", "/exit":
			fmt.Println("Bye!")
			os.Exit(0)

		default:
			fmt.Printf("%sUnknown: %s. Type /help%s\n", cR, cmd, cW)
		}
		return
	}

	// plain text → send to active
	c := getActive()
	if c == nil {
		fmt.Printf("%sNo active contact. Wait for a message first.%s\n", cR, cW)
		return
	}
	if err := msg.Text(cred, activeContact, c.contextToken, line); err != nil {
		fmt.Printf("%sSend failed: %s%s\n", cR, err, cW)
	} else {
		t := time.Now().Format("15:04:05")
		fmt.Printf("%s[%s] You → %s: %s%s\n", cDIM, t, c.name, line, cW)
	}
}

func Interactive(cred *api.Credential, profile string) {
	curProfile = profile
	fmt.Printf("\n%s%swx-cli%s Interactive Mode", cB, cG, cW)
	if profile != "" {
		fmt.Printf(" [%s]", profile)
	}
	fmt.Println()
	fmt.Printf("%sListening for messages... Type /help for commands.%s\n\n", cDIM, cW)

	done := make(chan struct{})
	go msg.Start(cred, profile, func(m *api.WeixinMessage) {
		displayMsg(m)
		fmt.Printf("%swx>%s ", cDIM, cW)
	}, done)

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Printf("%swx>%s ", cDIM, cW)
	for scanner.Scan() {
		handleLine(scanner.Text(), cred)
		fmt.Printf("%swx>%s ", cDIM, cW)
	}
	close(done)
}
