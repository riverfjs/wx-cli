package serve

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type tokenEntry struct {
	profile   string
	expiresAt time.Time // zero = never expires
}

var (
	pushTokens   = make(map[string]*tokenEntry)
	pushTokensMu sync.RWMutex
)

func generateToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ttl=0 means no expiry (bot process). ttl>0 for short-lived tokens (crontab).
func registerPushToken(profile string, ttl time.Duration) string {
	pushTokensMu.Lock()
	defer pushTokensMu.Unlock()

	token := generateToken()
	var exp time.Time
	if ttl > 0 {
		exp = time.Now().Add(ttl)
	}
	pushTokens[token] = &tokenEntry{profile: profile, expiresAt: exp}
	return token
}

func lookupPushToken(token string) (profile string, ok bool) {
	pushTokensMu.RLock()
	defer pushTokensMu.RUnlock()
	e, ok := pushTokens[token]
	if !ok {
		return "", false
	}
	if !e.expiresAt.IsZero() && time.Now().After(e.expiresAt) {
		return "", false
	}
	return e.profile, true
}

func cleanupExpiredTokens() {
	for {
		time.Sleep(time.Minute)
		pushTokensMu.Lock()
		now := time.Now()
		for tok, e := range pushTokens {
			if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
				delete(pushTokens, tok)
			}
		}
		pushTokensMu.Unlock()
	}
}
