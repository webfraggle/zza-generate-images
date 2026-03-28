package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/webfraggle/zza-generate-images/internal/cli"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "zza-desktop",
		Short: "Zugzielanzeiger image renderer (desktop CLI)",
	}
	root.AddCommand(cli.RenderCmd())
	return root
}
