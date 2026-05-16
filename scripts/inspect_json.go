//go:build ignore

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/AniruthKarthik/gramfix/internal/grammar"
)

func findJar() string {
	ltJarPaths := []string{
		"/usr/share/languagetool/languagetool-commandline.jar",
		"/usr/share/java/languagetool/languagetool-commandline.jar",
		"/opt/languagetool/languagetool-commandline.jar",
		"/usr/local/share/languagetool/languagetool-commandline.jar",
	}
	for _, p := range ltJarPaths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	patterns := []string{
		"/usr/**/languagetool-commandline.jar",
		"/opt/**/languagetool-commandline.jar",
	}
	for _, pat := range patterns {
		matches, _ := filepath.Glob(pat)
		if len(matches) > 0 {
			return matches[0]
		}
	}
	return ""
}

func main() {
	text := "The man who go to the shop are my friend."
	
	jar := findJar()
	if jar == "" {
		fmt.Println("JAR not found")
		os.Exit(1)
	}
	
	tmpFile, _ := os.CreateTemp("", "debug-*.txt")
	tmpFile.WriteString(text)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	cmd := exec.Command("java", "-jar", jar, "--language", "en-US", "--json", tmpFile.Name())
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Run()

	fmt.Println("Raw JSON Output:")
	fmt.Println(stdout.String())

	// Now try to apply it
	eng, _ := grammar.New("en-US")
	corrected, _ := eng.Fix(context.Background(), text)
	fmt.Printf("\nInput:  %q\nOutput: %q\n", text, corrected)
}
