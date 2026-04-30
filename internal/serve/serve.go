package serve

import (
	"fmt"
	"log"
	"net/http"
)

type Config struct {
	Port       int
	WxToken    string
	WxAppID    string
	WxSecret   string
	RootOpenID string
}

func Run(cfg Config) {
	if cfg.WxToken == "" {
		log.Fatal("[wx-serve] wx-token is required")
	}
	if cfg.WxAppID == "" || cfg.WxSecret == "" {
		log.Println("[wx-serve] wx-appid/wx-secret not set, async replies disabled")
	}
	if cfg.RootOpenID != "" {
		log.Printf("[wx-serve] root user: %s", cfg.RootOpenID)
	}

	go cleanupRecentMsgs()
	go cleanupExpiredTokens()

	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleRegister(w, r, &cfg)
	})

	http.HandleFunc("/push", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handlePush(w, r, &cfg)
	})

	http.HandleFunc("/relogin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleRelogin(w, r, &cfg)
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleVerify(w, r, cfg.WxToken)
		case http.MethodPost:
			handleMessage(w, r, &cfg)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	addr := fmt.Sprintf(":%d", cfg.Port)
	log.Printf("[wx-serve] listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
