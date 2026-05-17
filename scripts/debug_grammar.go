//go:build ignore

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/AniruthKarthik/gramfix/internal/env"
	"github.com/AniruthKarthik/gramfix/internal/grammar"
	"github.com/AniruthKarthik/gramfix/internal/log"
)

func main() {
	env.LoadDotEnv()
	log.SetDebug(true)

	cfg := grammar.DefaultEngineConfig()
	cfg.GroqAPIKey = os.Getenv("GROQ_API_KEY")

	eng, err := grammar.NewWithConfig(cfg)
	if err != nil {
		fmt.Printf("Error creating engine: %v\n", err)
		os.Exit(1)
	}

	testCases := []string{
		"i is man",
		"the man who go to the shop are my friend.",
	}

	models := []string{
		"meta-llama/llama-4-scout-17b-16e-instruct",
		"openai/gpt-oss-20b",
		"llama-3.1-8b-instant",
	}

	for _, model := range models {
		fmt.Printf("--- Testing Model: %s ---\n", model)
		cfg.GroqModel = model
		eng, err = grammar.NewWithConfig(cfg)
		if err != nil {
			fmt.Printf("Error updating engine: %v\n", err)
			continue
		}

		for _, tc := range testCases {
			corrected, err := eng.Fix(context.Background(), tc)
			if err != nil {
				fmt.Printf("Input: %q\nError: %v\n\n", tc, err)
				continue
			}
			fmt.Printf("Input:  %q\nOutput: %q\n\n", tc, corrected)
		}
	}
}
