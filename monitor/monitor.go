package monitor

import (
	"fmt"
	"os"
	"time"

	"wx-cli/api"
	"wx-cli/auth"
)

type Handler func(msg *api.WeixinMessage)

func Start(cred *api.Credential, handler Handler, done <-chan struct{}) {
	syncBuf := auth.LoadSyncBuf()
	errCount := 0

	for {
		select {
		case <-done:
			return
		default:
		}

		resp, err := api.GetUpdates(cred, syncBuf)
		if err != nil {
			errCount++
			if errCount > 10 {
				fmt.Fprintln(os.Stderr, "[monitor] Too many errors, stopping.")
				return
			}
			time.Sleep(3 * time.Second)
			continue
		}

		code := 0
		if resp.Ret != nil {
			code = *resp.Ret
		} else if resp.ErrCode != nil {
			code = *resp.ErrCode
		}

		if code == -14 {
			time.Sleep(5 * time.Second)
			continue
		}
		if code != 0 {
			fmt.Fprintf(os.Stderr, "[monitor] Error: ret=%d %s\n", code, resp.ErrMsg)
			errCount++
			time.Sleep(3 * time.Second)
			continue
		}

		errCount = 0
		if resp.GetUpdatesBuf != "" {
			syncBuf = resp.GetUpdatesBuf
			auth.SaveSyncBuf(syncBuf)
		}
		for _, msg := range resp.Msgs {
			handler(msg)
		}
	}
}
