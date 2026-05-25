package conf

import (
	"os/exec"
	"strings"
	"time"
)

var (
	Version   = "v1.9.7"
	Commit    = "unknown"
	BuildTime = "unknown"
	Author    = "lingyu"
	Repo      = "https://github.com/lingyuins/octopus"
)

func init() {
	if Commit == "unknown" {
		if out, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output(); err == nil {
			Commit = strings.TrimSpace(string(out))
		}
	}
	if BuildTime == "unknown" {
		BuildTime = time.Now().UTC().Format(time.RFC3339)
	}
}

