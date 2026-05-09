package main

import (
	"log"
	"os"

	"github.com/user/gramfix/internal/core"
)

func main() {
	log.SetFlags(0)
	
	if err := core.Run(); err != nil {
		log.Fatalf("Error: %v\n", err)
		os.Exit(1)
	}
}
