package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/stevehiehn/declaragent/internal/engine"
	"github.com/stevehiehn/declaragent/internal/plan"
)

var (
	runInputs  []string
	runApprove bool
)

var runCmd = &cobra.Command{
	Use:   "run <plan.yaml>",
	Short: "Execute a plan",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := plan.LoadFile(args[0])
		if err != nil {
			return err
		}
		inputs := parseInputs(runInputs)
		for name, inp := range p.Inputs {
			if _, ok := inputs[name]; !ok && inp.Default != "" {
				inputs[name] = inp.Default
			}
		}
		if err := plan.Validate(p, inputs); err != nil {
			return err
		}

		wd, _ := os.Getwd()
		ctx := engine.NewRunContext(wd, inputs, runApprove)
		result, err := engine.Execute(p, ctx, engine.ModeRun)
		if err != nil {
			return err
		}

		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(result)
		}

		if result.Success {
			fmt.Printf("Plan %q completed successfully.\n", p.Name)
		} else {
			fmt.Printf("Plan %q failed at step %q.\n", p.Name, result.FailedStepID)
			for _, e := range result.Errors {
				fmt.Printf("  Error: %s\n", e.Message)
				if e.Hint != "" {
					fmt.Printf("  Hint: %s\n", e.Hint)
				}
			}
		}
		fmt.Printf("Run ID: %s\n", result.RunID)
		return nil
	},
}

func init() {
	runCmd.Flags().StringArrayVar(&runInputs, "input", nil, "Input values (key=value)")
	runCmd.Flags().BoolVar(&runApprove, "approve", false, "Allow destructive steps")
	rootCmd.AddCommand(runCmd)
}
