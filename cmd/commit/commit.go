package commit

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"gitdiffy/api"
	"gitdiffy/git"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var CommitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Generate commit messages and confirm each before committing",
	Run: func(cmd *cobra.Command, args []string) {
		if viper.GetString("license") == "" {
			fmt.Println("❌ Please provide a license key using --license or .gitdiffy.yaml")
			os.Exit(1)
		}

		if !git.HasChanges() {
			fmt.Println("ℹ️ No changes to commit.")
			return
		}

		exec.Command("git", "add", ".").Run()

		diffOutput, err := exec.Command("git", "diff", "--cached").Output()
		if err != nil || len(diffOutput) == 0 {
			fmt.Println("ℹ️ No staged changes found.")
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

		exec.Command("git", "restore", "--staged", ".").Run()

		for _, commit := range commits {
			fmt.Printf("📦 Proposed commit message:\n  %s\n🗂 Affected files: %v\n", commit.Message, commit.Files)
			fmt.Print("✅ Do you want to commit this? (y/N): ")

			var input string
			fmt.Scanln(&input)
			if strings.ToLower(input) == "y" {
				args := append([]string{"add"}, commit.Files...)
				exec.Command("git", args...).Run()
				err := exec.Command("git", "commit", "-m", commit.Message).Run()
				if err != nil {
					fmt.Printf("❌ Failed to commit: %v\n", err)
				} else {
					fmt.Println("✅ Commit done.")
				}
			} else {
				fmt.Println("⏭ Skipping commit.")
			}
		}
	},
}
