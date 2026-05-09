package core

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/user/gramfix/internal/adapters/platform"
)

func Run() error {
	// Setup logging to file for debugging background execution
	logFile, _ := os.OpenFile("/tmp/gramfix.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if logFile != nil {
		defer logFile.Close()
		log.SetOutput(logFile)
	}
	log.Println("--- gramfix session started ---")

	// 0. Wait for hotkey (Alt+G) release to avoid modifier interference
	time.Sleep(300 * time.Millisecond)

	adapters, err := platform.Detect()
	if err != nil {
		log.Println("Detection error:", err)
		return err
	}

	log.Printf("Using Clipboard: %T\n", adapters.Clipboard)
	log.Printf("Using Injector: %T\n", adapters.Injector)

	// 1. Save current clipboard
	oldClip, _ := adapters.Clipboard.GetClipboard()

	// 2. Trigger Copy (Ctrl+C) to capture selection reliably
	log.Println("Triggering copy...")
	if err := adapters.Injector.SendCopy(); err != nil {
		log.Println("Copy trigger failed:", err)
		// Fallback to primary selection if Copy fails
		log.Println("Falling back to primary selection...")
	}
	
	// Give the system a moment to update clipboard
	time.Sleep(150 * time.Millisecond)

	// 3. Get text from clipboard
	text, err := adapters.Clipboard.GetClipboard()
	if err != nil {
		log.Println("Failed to get clipboard text:", err)
		return err
	}

	if strings.TrimSpace(text) == "" {
		log.Println("No text found in clipboard.")
		return nil
	}

	log.Printf("Captured text: %q\n", text)

	// 4. Correct text
	log.Println("Correcting text...")
	corrected, err := adapters.Grammar.Correct(text)
	if err != nil {
		log.Println("Grammar error:", err)
		return err
	}

	if corrected == text {
		log.Println("No corrections needed.")
		// Restore old clipboard just in case
		if oldClip != "" {
			_ = adapters.Clipboard.SetClipboard(oldClip)
		}
		return nil
	}

	log.Printf("Corrected text: %q\n", corrected)

	// 5. Set clipboard to corrected text
	if err := adapters.Clipboard.SetClipboard(corrected); err != nil {
		log.Println("Failed to set clipboard:", err)
		return err
	}

	// Wait for clipboard sync
	time.Sleep(150 * time.Millisecond)

	// 6. Inject Paste (Ctrl+V)
	log.Println("Injecting paste...")
	if err := adapters.Injector.SendPaste(); err != nil {
		log.Println("Paste injection failed:", err)
		return err
	}

	// Wait for paste to finish
	time.Sleep(300 * time.Millisecond)

	// 7. Restore original clipboard
	if oldClip != "" {
		log.Println("Restoring original clipboard...")
		_ = adapters.Clipboard.SetClipboard(oldClip)
	}

	log.Println("--- gramfix session finished successfully ---")
	return nil
}
