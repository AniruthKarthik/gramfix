package core

import (
	"errors"
	"log"
	"time"

	"github.com/user/gramfix/internal/adapters/platform"
)

func Run() error {
	adapters, err := platform.Detect()
	if err != nil {
		return err
	}

	log.Println("Using Clipboard:", adapters.Clipboard)
	log.Println("Using Injector:", adapters.Injector)

	// 1. Get primary selection (selected text)
	text, err := adapters.Clipboard.GetPrimary()
	if err != nil {
		return errors.New("failed to get selected text: " + err.Error())
	}

	if text == "" {
		log.Println("No text selected.")
		return nil
	}

	log.Printf("Selected text: %q\n", text)

	// 2. Correct text using Grammar Engine
	log.Println("Correcting text...")
	corrected, err := adapters.Grammar.Correct(text)
	if err != nil {
		return errors.New("grammar correction failed: " + err.Error())
	}

	if corrected == text {
		log.Println("No corrections needed.")
		return nil
	}

	log.Printf("Corrected text: %q\n", corrected)

	// 3. Save current clipboard
	oldClip, _ := adapters.Clipboard.GetClipboard()

	// 4. Set clipboard to corrected text
	if err := adapters.Clipboard.SetClipboard(corrected); err != nil {
		return errors.New("failed to set clipboard: " + err.Error())
	}

	// Give the clipboard a tiny moment to sync
	time.Sleep(100 * time.Millisecond)

	// 5. Inject Paste (Ctrl+V)
	log.Println("Injecting paste...")
	if err := adapters.Injector.SendPaste(); err != nil {
		return errors.New("failed to inject text: " + err.Error())
	}

	// Give the system a moment to process the paste before restoring clipboard
	time.Sleep(200 * time.Millisecond)

	// 6. Restore original clipboard
	if oldClip != "" {
		_ = adapters.Clipboard.SetClipboard(oldClip)
	}

	log.Println("Done.")
	return nil
}
