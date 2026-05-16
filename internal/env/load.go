package env

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/AniruthKarthik/gramfix/internal/log"
)

// LoadDotEnv searches for a .env file in standard locations and loads
// its variables into the process environment.
func LoadDotEnv() {
	home, _ := os.UserHomeDir()
	configDir := filepath.Join(home, ".config", "gramfix")

	paths := []string{
		".env",                               // Current directory
		"/home/ani/gramfix/.env",             // Project directory
		filepath.Join(home, ".env"),          // Home directory
		filepath.Join(configDir, ".env"),     // Config directory
	}

	for _, p := range paths {
		loadFromFile(p)
	}
}

func loadFromFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	log.Debug("loading environment from %s", path)

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		// Handle quotes
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}

		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}
