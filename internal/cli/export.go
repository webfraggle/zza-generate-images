package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/webfraggle/zza-generate-images/internal/devexport"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

// ExportCmd returns the "export" command: it writes a template as a ZZA
// controller package (<out>.zip). An incompatible template is not written;
// every reason goes to stderr and the exit status is non-zero.
func ExportCmd() *cobra.Command {
	var templateName, out, templatesDir string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export a template as a ZIP package for the ZZA controller",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := renderer.ValidateTemplateName(templateName); err != nil {
				return fmt.Errorf("export: %w", err)
			}
			if out == "" {
				out = templateName
			}
			if !strings.HasSuffix(strings.ToLower(out), ".zip") {
				out += ".zip"
			}
			pkg, err := devexport.Export(templatesDir, templateName)
			var inc *devexport.Incompatible
			if errors.As(err, &inc) {
				fmt.Fprintln(cmd.ErrOrStderr(), "nicht ZZA-kompatibel:")
				for _, r := range inc.Reasons {
					fmt.Fprintln(cmd.ErrOrStderr(), "  - "+r)
				}
				cmd.SilenceUsage = true
				return fmt.Errorf("export: %s ist nicht ZZA-kompatibel (%d Gründe)", templateName, len(inc.Reasons))
			}
			if err != nil {
				return fmt.Errorf("export: %w", err)
			}
			z, err := pkg.ZIP()
			if err != nil {
				return fmt.Errorf("export: %w", err)
			}
			if err := os.WriteFile(out, z, 0o644); err != nil {
				return fmt.Errorf("export: writing %s: %w", out, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Exported %s to %s (%d bytes)\n", templateName, out, len(z))
			return nil
		},
	}
	cmd.Flags().StringVarP(&templateName, "template", "t", "", "Template name (required)")
	cmd.Flags().StringVarP(&out, "output", "o", "", "Output file; .zip is appended if missing (default: <template>.zip)")
	cmd.Flags().StringVar(&templatesDir, "templates-dir", "./templates", "Templates directory")
	_ = cmd.MarkFlagRequired("template")
	return cmd
}
