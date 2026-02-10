package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version   = "dev"
	gitCommit = "unknown"
	buildDate = "unknown"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pskctl",
		Short: "PromSketch Control - CLI tooling for PromSketch-Dropin",
		Long: `pskctl is a command-line tool for managing and testing PromSketch-Dropin.

It provides utilities for:
  - Backfilling historical data
  - Running benchmarks (throughput and accuracy)
  - Validating configuration files
  - Managing PromSketch instances`,
		SilenceUsage: true,
	}

	// Add subcommands
	cmd.AddCommand(newVersionCmd())
	cmd.AddCommand(newCheckCmd())
	cmd.AddCommand(newBackfillCmd())
	cmd.AddCommand(newBenchCmd())

	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("pskctl\n")
			fmt.Printf("  version:    %s\n", version)
			fmt.Printf("  git commit: %s\n", gitCommit)
			fmt.Printf("  build date: %s\n", buildDate)
		},
	}
}
