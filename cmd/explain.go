package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/stevehiehn/declaragent/internal/engine"
	"github.com/stevehiehn/declaragent/internal/plan"
)

var explainInputs []string

var explainCmd = &cobra.Command{
	Use:   "explain <plan.yaml>",
	Short: "Show resolved plan steps without executing",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := plan.LoadFile(args[0])
		if err != nil {
			return err
		}
		inputs := parseInputs(explainInputs)
		// Apply defaults
		for name, inp := range p.Inputs {
			if _, ok := inputs[name]; !ok && inp.Default != "" {
				inputs[name] = inp.Default
			}
		}
		if err := plan.Validate(p, inputs); err != nil {
			return err
		}

		ctx := engine.NewRunContext(".", inputs, false)
		result, err := engine.Execute(p, ctx, engine.ModeExplain)
		if err != nil {
			return err
		}

		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(result)
		}

		fmt.Printf("Plan: %s\n", p.Name)
		if p.Description != "" {
			fmt.Printf("  %s\n", p.Description)
		}
		fmt.Println()
		for _, sr := range result.Steps {
			fmt.Printf("Step: %s\n", sr.ID)
			if sr.Description != "" {
				fmt.Printf("  Description: %s\n", sr.Description)
			}
			if sr.Command != "" {
				fmt.Printf("  Command: %s\n", sr.Command)
			}
			if sr.DryRunInfo != "" {
				fmt.Printf("  Info: %s\n", sr.DryRunInfo)
			}
			fmt.Println()
		}
		return nil
	},
}

func init() {
	explainCmd.Flags().StringArrayVar(&explainInputs, "input", nil, "Input values (key=value)")
	rootCmd.AddCommand(explainCmd)
}
