package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type GenerateMessageRequest struct {
	LicenseKey string `json:"licenseKey"`
	Diff       string `json:"diff"`
}

type Commit struct {
	Message string   `json:"message"`
	Files   []string `json:"files"`
}

type GenerateMessageResponse struct {
	Commits struct {
		Commits []Commit `json:"commits"`
	} `json:"commits"`
	Error string `json:"error,omitempty"`
}

var (
	licenseKey      string
	branchPrefix    string
	pushRemote      string
	maxWorkDuration time.Duration
	configFile      string
)

func main() {
	cobra.OnInitialize(initConfig)

	rootCmd := &cobra.Command{
		Use:   "gitdiffy",
		Short: "Gitdiffy automates smart commits based on code activity",
	}

	rootCmd.PersistentFlags().StringVarP(&licenseKey, "license", "l", "", "License key")
	rootCmd.PersistentFlags().StringVarP(&branchPrefix, "prefix", "p", "gitdiffy", "Temporary branch prefix")
	rootCmd.PersistentFlags().StringVar(&pushRemote, "pushRemote", "origin", "Git remote to push to")
	rootCmd.PersistentFlags().StringVar(&configFile, "config", "", "Config file (default: ./.gitdiffy.yaml)")
	rootCmd.PersistentFlags().DurationVar(&maxWorkDuration, "maxWorkDuration", 10*time.Minute, "Max time to work before triggering commit")

	viper.BindPFlags(rootCmd.PersistentFlags())

	rootCmd.AddCommand(watchCmd())

	rootCmd.Execute()
}

func initConfig() {
	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		viper.SetConfigName(".gitdiffy")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
	}

	viper.AutomaticEnv()
	viper.ReadInConfig()

	licenseKey = viper.GetString("license")
	branchPrefix = viper.GetString("prefix")
	pushRemote = viper.GetString("pushRemote")
	maxWorkDuration = viper.GetDuration("maxWorkDuration")
}

func watchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "watch",
		Short: "Watch the repo periodically and commit automatically",
		Run: func(cmd *cobra.Command, args []string) {
			if licenseKey == "" {
				fmt.Println("❌ Please provide a license key using --license or .gitdiffy.yaml")
				os.Exit(1)
			}
			fmt.Printf("🔁 Monitoring repo. Commit will be triggered after %v of continuous work.\n", maxWorkDuration)
			monitorRepo()
		},
	}
}

func monitorRepo() {
	ticker := time.NewTicker(time.Second)
	var firstChange time.Time
	s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
	s.Suffix = " Waiting for enough changes..."
	s.Start()

	for range ticker.C {
		if hasChanges() {
			if firstChange.IsZero() {
				firstChange = time.Now()
			}
			elapsed := time.Since(firstChange)
			remaining := max(maxWorkDuration-elapsed, 0)
			s.Suffix = fmt.Sprintf(" ⏳ Next auto commit in %v...", remaining.Truncate(time.Second))

			if elapsed >= maxWorkDuration {
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

func hasChanges() bool {
	out, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil {
		fmt.Println("❌ git status error:", err)
		return false
	}
	return strings.TrimSpace(string(out)) != ""
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
	commits, err := generateCommitMessages(string(diffOutput))
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
	if !strings.HasPrefix(currentBranch, branchPrefix) {
		timestamp := time.Now().Format("20060102-150405")
		branchName = fmt.Sprintf("%s-%s", branchPrefix, timestamp)
		exec.Command("git", "checkout", "-b", branchName).Run()
	}
	exec.Command("git", "restore", "--staged", ".").Run()

	for _, commit := range commits {
		fmt.Printf("📦 %s (files: %v)\n", commit.Message, commit.Files)
		args := append([]string{"add"}, commit.Files...)
		exec.Command("git", args...).Run()
		exec.Command("git", "commit", "-m", commit.Message).Run()
	}

	exec.Command("git", "push", "-u", pushRemote, branchName).Run()

	fmt.Println("✅ Commit and push complete to branch:", branchName)
}

func generateCommitMessages(diff string) ([]Commit, error) {
	req := GenerateMessageRequest{LicenseKey: licenseKey, Diff: diff}
	body, _ := json.Marshal(req)

	resp, err := http.Post("http://localhost:8080/generate-message", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	var res GenerateMessageResponse
	json.Unmarshal(data, &res)

	if res.Error != "" {
		return nil, fmt.Errorf("API error: %s", res.Error)
	}
	return res.Commits.Commits, nil
}

func max(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}
