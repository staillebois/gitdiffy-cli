package watch

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"gitdiffy/api"
	"gitdiffy/git"
)

var WatchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch the repo periodically and commit automatically",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("🔁 Monitoring repo. Commit will be triggered after %v of continuous work.\n", viper.GetDuration("maxWorkDuration"))
		monitorRepo()
	},
}

func monitorRepo() {
	ticker := time.NewTicker(time.Second)
	var firstChange time.Time
	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	s.Suffix = " Waiting for enough changes..."
	s.Start()

	for range ticker.C {
		if git.HasChanges() {
			if firstChange.IsZero() {
				firstChange = time.Now()
			}
			elapsed := time.Since(firstChange)
			remaining := max(viper.GetDuration("maxWorkDuration")-elapsed, 0)
			s.Suffix = fmt.Sprintf(" ⏳ Next auto commit in %v...", remaining.Truncate(time.Second))

			if elapsed >= viper.GetDuration("maxWorkDuration") {
				s.Stop()
				fmt.Println("⏱ Triggering commit after work duration exceeded...")
				performAutoCommit()
				firstChange = time.Time{}
				s.Start()
			}
		} else {
			firstChange = time.Time{}
			s.Suffix = " Waiting for enough changes..."
		}
	}
}

func performAutoCommit() {
	exec.Command("git", "add", ".").Run()
	statusOutput, err := exec.Command("git", "status", "--short").Output()
	if err == nil && len(statusOutput) > 0 {
		fmt.Println("🗂 Files staged for commit:")
		lines := strings.Split(string(statusOutput), "\n")
		for _, line := range lines {
			if line == "" {
				continue
			}
			prefix := strings.TrimSpace(line[:2])
			file := strings.TrimSpace(line[2:])
			icon := "📄"
			switch {
			case strings.HasPrefix(prefix, "A"):
				icon = "🟢"
			case strings.HasPrefix(prefix, "M"):
				icon = "🟡"
			case strings.HasPrefix(prefix, "D"):
				icon = "🔴"
			case strings.HasPrefix(prefix, "??"):
				icon = "✨"
			}
			fmt.Printf("  %s %s\n", icon, file)
		}
	}

	diffOutput, err := exec.Command("git", "diff", "--cached").Output()
	if err != nil || len(diffOutput) == 0 {
		fmt.Println("ℹ️ No changes to commit.")
		return
	}

	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	s.Suffix = " Generating commit messages..."
	s.Start()
	commits, err := api.GenerateCommitMessages(string(diffOutput))
	s.Stop()
	if err != nil {
		fmt.Println("❌", err)
		return
	}

	currentBranch := "main"
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		fmt.Println("❌ failed to get current branch", err)
	} else {
		currentBranch = strings.TrimSpace(string(out))
	}

	branchName := currentBranch
	if !strings.HasPrefix(currentBranch, viper.GetString("branchPrefix")) {
		timestamp := time.Now().Format("20060102-150405")
		branchName = fmt.Sprintf("%s-%s", viper.GetString("branchPrefix"), timestamp)
		exec.Command("git", "checkout", "-b", branchName).Run()
	}
	exec.Command("git", "restore", "--staged", ".").Run()

	for _, commit := range commits {
		fmt.Printf("📦 %s (files: %v)\n", commit.Message, commit.Files)
		args := append([]string{"add"}, commit.Files...)
		exec.Command("git", args...).Run()
		exec.Command("git", "commit", "-m", commit.Message).Run()
	}

	exec.Command("git", "push", "-u", viper.GetString("pushRemote"), branchName).Run()

	fmt.Println("✅ Commit and push complete to branch:", branchName)
}

func max(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}
