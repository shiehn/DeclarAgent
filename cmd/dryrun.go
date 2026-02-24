package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/stevehiehn/declaragent/internal/engine"
	"github.com/stevehiehn/declaragent/internal/plan"
)

var dryRunInputs []string

var dryRunCmd = &cobra.Command{
	Use:   "dry-run <plan.yaml>",
	Short: "Show what would be executed without running",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := plan.LoadFile(args[0])
		if err != nil {
			return err
		}
		inputs := parseInputs(dryRunInputs)
		for name, inp := range p.Inputs {
			if _, ok := inputs[name]; !ok && inp.Default != "" {
				inputs[name] = inp.Default
			}
		}
		if err := plan.Validate(p, inputs); err != nil {
			return err
		}

		ctx := engine.NewRunContext(".", inputs, false)
		result, err := engine.Execute(p, ctx, engine.ModeDryRun)
		if err != nil {
			return err
		}

		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(result)
		}

		fmt.Printf("Dry-run: %s\n\n", p.Name)
		for _, sr := range result.Steps {
			fmt.Printf("Step: %s [%s]\n", sr.ID, sr.Status)
			if sr.DryRunInfo != "" {
				fmt.Printf("  %s\n", sr.DryRunInfo)
			}
			if sr.Command != "" && sr.DryRunInfo == "" {
				fmt.Printf("  Would run: %s\n", sr.Command)
			}
			fmt.Println()
		}
		return nil
	},
}

func init() {
	dryRunCmd.Flags().StringArrayVar(&dryRunInputs, "input", nil, "Input values (key=value)")
	rootCmd.AddCommand(dryRunCmd)
}
