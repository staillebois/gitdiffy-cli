package cmd

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"gitdiffy/cmd/commit"
	"gitdiffy/cmd/watch"
)

var (
	licenseKey      string
	branchPrefix    string
	pushRemote      string
	maxWorkDuration time.Duration
	configFile      string

	RootCmd = &cobra.Command{
		Use:   "gitdiffy",
		Short: "Gitdiffy automates smart commits based on code activity",
	}
)

func Execute() error {
	return RootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	RootCmd.PersistentFlags().StringVarP(&licenseKey, "license", "l", "", "License key")
	RootCmd.PersistentFlags().StringVarP(&branchPrefix, "branchPrefix", "p", "gitdiffy", "Temporary branch prefix")
	RootCmd.PersistentFlags().StringVar(&pushRemote, "pushRemote", "origin", "Git remote to push to")
	RootCmd.PersistentFlags().StringVar(&configFile, "config", "", "Config file (default: ./.gitdiffy.yaml)")
	RootCmd.PersistentFlags().DurationVar(&maxWorkDuration, "maxWorkDuration", 10*time.Minute, "Max time to work before triggering commit")

	viper.BindPFlags(RootCmd.PersistentFlags())

	RootCmd.AddCommand(commit.CommitCmd)
	RootCmd.AddCommand(watch.WatchCmd)
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
	branchPrefix = viper.GetString("branchPrefix")
	pushRemote = viper.GetString("pushRemote")
	maxWorkDuration = viper.GetDuration("maxWorkDuration")
}
