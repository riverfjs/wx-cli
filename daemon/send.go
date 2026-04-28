package daemon

import (
	"fmt"
	"os"

	"wx-cli/api"
	"wx-cli/send"
)

func Send(cred *api.Credential, args []string) {
	var to, ctx, text, image, file, video string

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

	if to == "" || ctx == "" {
		fmt.Fprintln(os.Stderr, "Usage: wx send --to USER_ID --ctx CONTEXT_TOKEN --text MSG")
		fmt.Fprintln(os.Stderr, "       wx send --to USER_ID --ctx CONTEXT_TOKEN --image PATH")
		fmt.Fprintln(os.Stderr, "       wx send --to USER_ID --ctx CONTEXT_TOKEN --file PATH")
		os.Exit(1)
	}

	var err error
	switch {
	case text != "":
		err = send.Text(cred, to, ctx, text)
	case image != "":
		err = send.Image(cred, to, ctx, image)
	case file != "":
		err = send.File(cred, to, ctx, file)
	case video != "":
		err = send.Video(cred, to, ctx, video)
	default:
		fmt.Fprintln(os.Stderr, "Specify --text, --image, --file, or --video")
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Send failed: %s\n", err)
		os.Exit(1)
	}
}
