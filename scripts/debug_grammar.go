package main

import (
	"context"
	"fmt"
	"os"

	"github.com/anilnair00/gramfix/internal/grammar"
	"github.com/anilnair00/gramfix/internal/log"
)

func main() {
	log.SetDebug(true)
	eng, err := grammar.New("en-US")
	if err != nil {
		fmt.Printf("Error creating engine: %v\n", err)
		os.Exit(1)
	}

	testCases := []string{
		"i is man",
		"the man who go to the shop are my friend.",
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
