/*
Copyright © 2025-2026 Artur Taranchiev <artur.taranchiev@gmail.com>
SPDX-License-Identifier: Apache-2.0
*/
package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"ezbk-tui/internal/ezbk"
	"ezbk-tui/internal/logging"
	"ezbk-tui/internal/ui"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "ezbk-tui",
	Short: "A TUI for the ezBookkeeping personal finance app",
	Long: `ezbk-tui is a terminal user interface for ezBookkeeping.
It lets you view and manage accounts, categories, tags and transactions from the terminal.

Prerequisites:
  - A running ezBookkeeping instance with API tokens enabled (enable_api_token = true).
  - An API token generated in User Settings -> Security (or with the user-session-new CLI command).`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initializeConfig(cmd)
	},
	RunE: run,
}

func run(cmd *cobra.Command, args []string) error {
	debug := viper.GetBool("logging.debug")
	logFile := viper.GetString("logging.file")

	var (
		logger  *zap.Logger
		cleanup func()
		err     error
	)
	if logFile == "" {
		logger, cleanup, err = logging.New(debug)
	} else {
		logger, cleanup, err = logging.New(debug, logFile)
	}
	if err != nil {
		return fmt.Errorf("failed to init logger: %w", err)
	}
	defer cleanup()
	zap.ReplaceGlobals(logger)

	cfg := ezbk.Config{
		ApiUrl:         viper.GetString("ezbk.api_url"),
		Token:          viper.GetString("ezbk.token"),
		Timezone:       viper.GetString("ezbk.timezone"),
		TimeoutSeconds: viper.GetInt("timeout"),
	}
	if cfg.Token == "" {
		return errors.New("ezBookkeeping API token is not set (ezbk.token)")
	}
	if cfg.ApiUrl == "" {
		return errors.New("ezBookkeeping URL is not set (ezbk.api_url)")
	}

	api, err := ezbk.NewApi(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to ezBookkeeping: %w", err)
	}
	logger.Info("Connected to ezBookkeeping",
		zap.String("api_url", cfg.ApiUrl),
		zap.String("user", api.Username()))

	ui.Show(api)

	if cfgUsed := viper.ConfigFileUsed(); cfgUsed != "" {
		viper.Set("logging.debug", false)
		if err := viper.WriteConfigAs(cfgUsed); err != nil {
			zap.L().Warn("Failed to persist configuration on exit",
				zap.String("config_file", cfgUsed),
				zap.Error(err))
		}
	}
	return nil
}

var initConfigCmd = &cobra.Command{
	Use:   "init-config",
	Short: "Generate a default configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		initViper := viper.New()
		initViper.Set("ezbk.api_url", viper.GetString("ezbk.api_url"))
		initViper.Set("ezbk.token", viper.GetString("ezbk.token"))
		initViper.Set("ezbk.timezone", viper.GetString("ezbk.timezone"))
		initViper.SetConfigFile("./config.yaml")

		if err := initViper.SafeWriteConfig(); err != nil {
			var exists viper.ConfigFileAlreadyExistsError
			if errors.As(err, &exists) {
				return err
			}
		}
		fmt.Println("Configuration file created at:", initViper.ConfigFileUsed())
		return nil
	},
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/ezbk-tui/config.yaml)")
	rootCmd.PersistentFlags().StringP("ezbk.api_url", "u", "https://your-ezbookkeeping-instance.com", "ezBookkeeping base URL")
	rootCmd.PersistentFlags().StringP("ezbk.token", "k", "", "ezBookkeeping API token")
	rootCmd.PersistentFlags().StringP("ezbk.timezone", "z", "", "IANA timezone name (default: system local)")
	rootCmd.PersistentFlags().IntP("timeout", "t", 10, "Connection timeout in seconds")
	rootCmd.Flags().BoolP("logging.debug", "d", false, "Enable debug logging")
	rootCmd.Flags().StringP("logging.file", "l", "", "Log file path (if empty, logs to stdout)")

	rootCmd.AddCommand(initConfigCmd)
}

func initializeConfig(cmd *cobra.Command) error {
	viper.SetEnvPrefix("EZBK_TUI")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.AutomaticEnv()
	viper.SetDefault("ui.convert_totals", true)

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		viper.AddConfigPath(".")
		viper.AddConfigPath(home + "/.config/ezbk-tui")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return fmt.Errorf("failed to read config: %w", err)
		}
	} else {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}

	if err := viper.BindPFlags(cmd.Flags()); err != nil {
		return fmt.Errorf("failed to bind flags: %w", err)
	}
	return nil
}
