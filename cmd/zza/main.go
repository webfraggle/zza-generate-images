package main

import (
	"encoding/json"
	"fmt"
	"image/png"
	"os"

	"github.com/spf13/cobra"
	"github.com/webfraggle/zza-generate-images/internal/renderer"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "zza",
		Short: "Zugzielanzeiger image generator",
	}
	root.AddCommand(renderCmd())
	return root
}

func renderCmd() *cobra.Command {
	var (
		templateName string
		inputFile    string
		outputFile   string
		templatesDir string
	)

	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render a template with JSON input data to a PNG image",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate template name.
			if err := renderer.ValidateTemplateName(templateName); err != nil {
				return fmt.Errorf("render: %w", err)
			}

			// Read and parse JSON input.
			jsonBytes, err := os.ReadFile(inputFile)
			if err != nil {
				return fmt.Errorf("render: reading input file: %w", err)
			}
			var data map[string]interface{}
			if err := json.Unmarshal(jsonBytes, &data); err != nil {
				return fmt.Errorf("render: parsing JSON: %w", err)
			}

			// Load template.
			tmpl, err := renderer.LoadTemplate(templatesDir, templateName)
			if err != nil {
				return fmt.Errorf("render: loading template: %w", err)
			}

			// Render.
			r := renderer.New(templatesDir)
			img, err := r.Render(tmpl, data)
			if err != nil {
				return fmt.Errorf("render: rendering: %w", err)
			}

			// Write PNG output.
			outF, err := os.Create(outputFile)
			if err != nil {
				return fmt.Errorf("render: creating output file: %w", err)
			}

			if err := png.Encode(outF, img); err != nil {
				outF.Close()
				os.Remove(outputFile) // clean up partial file on encode error
				return fmt.Errorf("render: encoding PNG: %w", err)
			}

			if err := outF.Close(); err != nil {
				os.Remove(outputFile)
				return fmt.Errorf("render: closing output file: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Rendered %s to %s\n", templateName, outputFile)
			return nil
		},
	}

	cmd.Flags().StringVarP(&templateName, "template", "t", "", "Template name (required)")
	cmd.Flags().StringVarP(&inputFile, "input", "i", "", "Input JSON file (required)")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output PNG file (required)")
	cmd.Flags().StringVar(&templatesDir, "templates-dir", "./templates", "Templates directory")

	_ = cmd.MarkFlagRequired("template")
	_ = cmd.MarkFlagRequired("input")
	_ = cmd.MarkFlagRequired("output")

	return cmd
}
