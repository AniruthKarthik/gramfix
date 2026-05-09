package grammar

import (
	"bytes"
	"encoding/xml"
	"errors"
	"os"
	"os/exec"
	"sort"
	"strings"
)

type LanguageToolEngine struct{}

func (e *LanguageToolEngine) IsAvailable() bool {
	_, err := exec.LookPath("languagetool")
	if err == nil {
		return true
	}
	_, err = os.Stat("/usr/share/languagetool/languagetool-commandline.jar")
	return err == nil
}

type Matches struct {
	Errors []Error `xml:"error"`
}

type Error struct {
	Offset       int    `xml:"offset,attr"`
	ErrorLength  int    `xml:"errorlength,attr"`
	Replacements string `xml:"replacements,attr"`
}

func (e *LanguageToolEngine) Correct(text string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return text, nil
	}

	var cmd *exec.Cmd
	if _, err := exec.LookPath("languagetool"); err == nil {
		cmd = exec.Command("languagetool", "--api", "-l", "en-US", "-")
	} else {
		cmd = exec.Command("java", "-jar", "/usr/share/languagetool/languagetool-commandline.jar", "--api", "-l", "en-US", "-")
	}

	cmd.Stdin = strings.NewReader(text)
	var out bytes.Buffer
	cmd.Stdout = &out
	
	err := cmd.Run()
	// languagetool might exit with non-zero if errors are found, we still parse stdout.
	if err != nil && out.Len() == 0 {
		return "", errors.New("languagetool failed to execute: " + err.Error())
	}

	output := out.String()
	xmlStart := strings.Index(output, "<?xml")
	if xmlStart == -1 {
		return text, nil // No valid XML output
	}
	xmlData := output[xmlStart:]

	var matches Matches
	if err := xml.Unmarshal([]byte(xmlData), &matches); err != nil {
		return "", err
	}

	// Apply corrections from back to front to avoid shifting offsets
	sort.Slice(matches.Errors, func(i, j int) bool {
		return matches.Errors[i].Offset > matches.Errors[j].Offset
	})

	runes := []rune(text)
	for _, err := range matches.Errors {
		reps := strings.Split(err.Replacements, "#")
		if len(reps) > 0 && reps[0] != "" {
			replacement := reps[0]
			start := err.Offset
			end := start + err.ErrorLength
			// Ensure offsets are within bounds (languagetool might use byte or char offsets, 
			// assuming char/rune offsets)
			if start >= 0 && end <= len(runes) && start <= end {
				prefix := runes[:start]
				suffix := runes[end:]
				runes = append(append(prefix, []rune(replacement)...), suffix...)
			}
		}
	}

	return string(runes), nil
}
