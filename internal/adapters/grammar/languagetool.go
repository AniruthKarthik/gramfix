package grammar

import (
	"bytes"
	"encoding/json"
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
	if err == nil {
		return true
	}
	_, err = os.Stat("tmp/lt/languagetool-commandline.jar")
	return err == nil
}

type LTResponse struct {
	Matches []LTMatch `json:"matches"`
}

type LTMatch struct {
	Offset       int             `json:"offset"`
	Length       int             `json:"length"`
	Replacements []LTReplacement `json:"replacements"`
}

type LTReplacement struct {
	Value string `json:"value"`
}

func (e *LanguageToolEngine) Correct(text string) (string, error) {
	if strings.TrimSpace(text) == "" {
		return text, nil
	}

	var cmd *exec.Cmd
	if _, err := exec.LookPath("languagetool"); err == nil {
		cmd = exec.Command("languagetool", "--json", "-l", "en-US", "-")
	} else if _, err := os.Stat("/usr/share/languagetool/languagetool-commandline.jar"); err == nil {
		cmd = exec.Command("java", "-jar", "/usr/share/languagetool/languagetool-commandline.jar", "--json", "-l", "en-US", "-")
	} else {
		cmd = exec.Command("java", "-jar", "tmp/lt/languagetool-commandline.jar", "--json", "-l", "en-US", "-")
	}

	cmd.Stdin = strings.NewReader(text)
	var out bytes.Buffer
	cmd.Stdout = &out
	
	err := cmd.Run()
	if err != nil && out.Len() == 0 {
		return "", errors.New("languagetool failed to execute: " + err.Error())
	}

	output := out.String()
	jsonStart := strings.Index(output, "{")
	jsonEnd := strings.LastIndex(output, "}")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd < jsonStart {
		return text, nil
	}
	jsonData := output[jsonStart : jsonEnd+1]

	var resp LTResponse
	if err := json.Unmarshal([]byte(jsonData), &resp); err != nil {
		return "", err
	}

	matches := resp.Matches
	// Filter overlapping matches: keep the first match that covers a span
	var filtered []LTMatch
	used := make([]bool, len(text))
	for _, match := range matches {
		if len(match.Replacements) == 0 || match.Replacements[0].Value == "" {
			continue
		}
		start := match.Offset
		end := start + match.Length
		if start < 0 || end > len(text) {
			continue
		}
		
		overlap := false
		for i := start; i < end; i++ {
			if used[i] {
				overlap = true
				break
			}
		}
		if !overlap {
			filtered = append(filtered, match)
			for i := start; i < end; i++ {
				used[i] = true
			}
		}
	}

	// Apply corrections from back to front
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Offset > filtered[j].Offset
	})

	runes := []rune(text)
	for _, match := range filtered {
		replacement := match.Replacements[0].Value
		start := match.Offset
		end := start + match.Length
		// Re-validate bounds on runes just in case
		if start >= 0 && end <= len(runes) && start <= end {
			prefix := runes[:start]
			suffix := runes[end:]
			
			var newRunes []rune
			newRunes = append(newRunes, prefix...)
			newRunes = append(newRunes, []rune(replacement)...)
			newRunes = append(newRunes, suffix...)
			runes = newRunes
		}
	}

	return string(runes), nil
}
