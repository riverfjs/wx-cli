package serve

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

func handleRegister(w http.ResponseWriter, r *http.Request, cfg *Config) {
	profile := r.FormValue("profile")
	secret := r.FormValue("secret")
	ttlParam := r.FormValue("ttl")

	if profile == "" || secret == "" {
		http.Error(w, "missing profile or secret", http.StatusBadRequest)
		return
	}
	if secret != cfg.WxToken {
		http.Error(w, "invalid secret", http.StatusForbidden)
		return
	}

	var ttl time.Duration
	if ttlParam != "" {
		if d, err := time.ParseDuration(ttlParam); err == nil {
			ttl = d
		}
	}

	token := registerPushToken(profile, ttl)
	label := "permanent"
	if ttl > 0 {
		label = ttl.String()
	}
	log.Printf("[wx-serve] registered push token for %s (ttl=%s)", shortID(profile), label)
	fmt.Fprint(w, token)
}

func handlePush(w http.ResponseWriter, r *http.Request, cfg *Config) {
	r.ParseForm()
	token := r.FormValue("token")
	text := r.FormValue("text")
	tplName := r.FormValue("template")

	if token == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	alias, ok := lookupPushToken(token)
	if !ok {
		http.Error(w, "invalid or expired token", http.StatusForbidden)
		return
	}

	openID, ok := resolveAlias(alias)
	if !ok {
		http.Error(w, "profile mapping not found", http.StatusInternalServerError)
		return
	}

	var keywords []string
	var templateID string
	if tplName != "" {
		templateID = getTemplate(tplName)
		if templateID == "" {
			http.Error(w, "template not found: "+tplName, http.StatusBadRequest)
			return
		}
		for i := 1; ; i++ {
			v := r.FormValue(fmt.Sprintf("k%d", i))
			if v == "" {
				break
			}
			keywords = append(keywords, v)
		}
		if len(keywords) == 0 {
			http.Error(w, "missing keyword params (k1, k2, ...)", http.StatusBadRequest)
			return
		}
	} else if text == "" {
		http.Error(w, "missing template or text", http.StatusBadRequest)
		return
	}

	if templateID != "" {
		if err := sendTemplate(cfg, openID, templateID, keywords); err != nil {
			log.Printf("[wx-serve] push failed for %s: %s", alias, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		sendAsync(cfg, openID, text)
	}

	log.Printf("[wx-serve] push to=%s tpl=%s", alias, tplName)
	fmt.Fprint(w, "ok")
}
