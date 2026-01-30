package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/maximbilan/grammr/internal/config"
	"github.com/maximbilan/grammr/internal/ui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "grammr",
	Short: "Lightning-fast AI grammar checker",
	Long:  `grammr is a TUI grammar checker that uses OpenAI to fix your writing instantly.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := ui.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
}

var setCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set a config value",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		if err := config.Set(args[0], args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Set %s = %s\n", args[0], args[1])
	},
}

var getCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a config value",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		value := config.Get(args[0])
		fmt.Printf("%s = %v\n", args[0], value)
	},
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configuration file",
	Run: func(cmd *cobra.Command, args []string) {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		configPath := filepath.Join(home, ".grammr")
		if err := os.MkdirAll(configPath, 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		if err := config.Save(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Configuration initialized at %s\n", filepath.Join(configPath, "config.yaml"))
		fmt.Println("Set your API key with: grammr config set api_key YOUR_KEY")
	},
}

func init() {
	configCmd.AddCommand(setCmd)
	configCmd.AddCommand(getCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(initCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
