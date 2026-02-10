package main

import (
	"fmt"

	"github.com/promsketch/promsketch-dropin/internal/config"
	"github.com/spf13/cobra"
)

func newCheckCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Validate configuration files",
		Long:  "Validate PromSketch-Dropin configuration files for syntax and correctness",
	}

	cmd.AddCommand(newCheckConfigCmd())

	return cmd
}

func newCheckConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config [file]",
		Short: "Validate a configuration file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			configFile := args[0]

			fmt.Printf("Validating configuration file: %s\n", configFile)

			// Load and validate config
			cfg, err := config.LoadConfig(configFile)
			if err != nil {
				return fmt.Errorf("configuration validation failed: %w", err)
			}

			// Print summary
			fmt.Printf("✅ Configuration is valid\n\n")
			fmt.Printf("Summary:\n")
			fmt.Printf("  Listen address:    %s\n", cfg.Server.ListenAddress)
			fmt.Printf("  Backend type:      %s\n", cfg.Backend.Type)
			fmt.Printf("  Backend URL:       %s\n", cfg.Backend.URL)
			fmt.Printf("  Sketch partitions: %d\n", cfg.Sketch.NumPartitions)
			fmt.Printf("  Sketch targets:    %d\n", len(cfg.Sketch.Targets))
			fmt.Printf("  Remote write:      %v\n", cfg.Ingestion.RemoteWrite.Enabled)
			fmt.Printf("  Scrape enabled:    %v\n", cfg.Ingestion.Scrape.Enabled)

			return nil
		},
	}
}
