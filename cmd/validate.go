package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/stevehiehn/declaragent/internal/plan"
)

var validateCmd = &cobra.Command{
	Use:   "validate <plan.yaml>",
	Short: "Validate a plan file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		p, err := plan.LoadFile(args[0])
		if err != nil {
			return err
		}
		if err := plan.Validate(p, nil); err != nil {
			if jsonOutput {
				json.NewEncoder(os.Stdout).Encode(map[string]any{"valid": false, "error": err.Error()})
			} else {
				fmt.Fprintf(os.Stderr, "Validation failed: %s\n", err)
			}
			os.Exit(1)
		}
		if jsonOutput {
			json.NewEncoder(os.Stdout).Encode(map[string]any{"valid": true})
		} else {
			fmt.Println("Plan is valid.")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
