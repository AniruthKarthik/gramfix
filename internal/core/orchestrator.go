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

	// 0. Wait for hotkey release to avoid modifier interference
	time.Sleep(300 * time.Millisecond)

	adapters, err := platform.Detect()
	if err != nil {
		log.Println("Detection error:", err)
		return err
	}

	log.Printf("Using Clipboard: %T\n", adapters.Clipboard)
	log.Printf("Using Injector: %T\n", adapters.Injector)

	var text string
	var oldClip string
	var usedClipboard bool

	// 1. Try Primary Selection first (highlighted text)
	log.Println("Attempting to get Primary selection...")
	text, err = adapters.Clipboard.GetPrimary()
	if err == nil && strings.TrimSpace(text) != "" {
		log.Printf("Captured via Primary: %q\n", text)
	} else {
		log.Println("Primary selection empty or failed, trying Ctrl+C...")
		
		// 2. Save current clipboard to restore later
		oldClip, _ = adapters.Clipboard.GetClipboard()
		usedClipboard = true

		// 3. Trigger Copy (Ctrl+C)
		if err := adapters.Injector.SendCopy(); err != nil {
			log.Println("Copy trigger failed:", err)
		}
		
		// Give the system time to update clipboard
		time.Sleep(300 * time.Millisecond)

		text, err = adapters.Clipboard.GetClipboard()
		if err != nil {
			log.Println("Failed to get clipboard text:", err)
			return err
		}
	}

	if strings.TrimSpace(text) == "" {
		log.Println("No text found to correct.")
		return nil
	}

	log.Printf("Final Captured text: %q\n", text)

	// 4. Correct text
	log.Println("Correcting text...")
	corrected, err := adapters.Grammar.Correct(text)
	if err != nil {
		log.Println("Grammar error:", err)
		return err
	}

	if corrected == text {
		log.Println("No corrections needed.")
		return nil
	}

	log.Printf("Corrected text: %q\n", corrected)

	// 5. Set clipboard to corrected text (we always use clipboard for pasting)
	if err := adapters.Clipboard.SetClipboard(corrected); err != nil {
		log.Println("Failed to set clipboard:", err)
		return err
	}

	// Wait for clipboard sync
	time.Sleep(200 * time.Millisecond)

	// 6. Inject Paste (Ctrl+V)
	log.Println("Injecting paste...")
	if err := adapters.Injector.SendPaste(); err != nil {
		log.Println("Paste injection failed:", err)
		return err
	}

	// Wait for paste to finish before restoring
	time.Sleep(400 * time.Millisecond)

	// 7. Restore original clipboard if we used it or if we want to be clean
	if usedClipboard && oldClip != "" {
		log.Println("Restoring original clipboard...")
		_ = adapters.Clipboard.SetClipboard(oldClip)
	}

	log.Println("--- gramfix session finished successfully ---")
	return nil
}
