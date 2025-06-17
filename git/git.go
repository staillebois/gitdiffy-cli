package git

import (
	"fmt"
	"os/exec"
	"strings"
)

func HasChanges() bool {
	out, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil {
		fmt.Println("❌ git status error:", err)
		return false
	}
	return strings.TrimSpace(string(out)) != ""
}
