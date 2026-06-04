package main

import (

	"fmt"
	"os"

	"github.com/inovacc/daemon/internal/parameters"

	"github.com/inovacc/config"

	"github.com/spf13/cobra"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "daemon",
	Short: "daemon is a CLI application",
	Long: `daemon is a CLI application

This is a CLI application built with Cobra.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func main() {
	Execute()
}

func init() {

	cobra.OnInitialize(initConfig)


	rootCmd.Version = GetVersionJSON()
	rootCmd.CompletionOptions.DisableDefaultCmd = true


	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "config.yaml", "config file (default is config.yaml)")

}

// initConfig reads in config file and ENV variables if set.
func initConfig() {

	if cfgFile == "" {
		_, _ = fmt.Fprint(os.Stdout, "Using default config file: config.yaml")
	}

	// Load configuration from a file, applying defaults if needed
	if err := config.InitServiceConfig(&parameters.Service{}, cfgFile); err != nil {
		_, _ = fmt.Fprint(os.Stdout, "failed to load config: %w", err)
	}

}

