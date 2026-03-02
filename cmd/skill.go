package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/shiehn/declaragent/internal/plan"
	"github.com/shiehn/declaragent/internal/skill"
)

var (
	skillPlansDir string
	skillOutput   string
	skillBinary   string
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Generate a Claude Code Skill (SKILL.md) for DeclarAgent",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		var plans []skill.PlanInfo

		if skillPlansDir != "" {
			entries, err := os.ReadDir(skillPlansDir)
			if err != nil {
				return fmt.Errorf("reading plans directory: %w", err)
			}
			for _, e := range entries {
				if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
					continue
				}
				p, err := plan.LoadFile(filepath.Join(skillPlansDir, e.Name()))
				if err != nil {
					continue
				}
				plans = append(plans, skill.PlanInfo{
					Filename: e.Name(),
					Plan:     p,
				})
			}
		}

		if err := os.MkdirAll(skillOutput, 0o755); err != nil {
			return fmt.Errorf("creating output directory: %w", err)
		}

		outPath := filepath.Join(skillOutput, "SKILL.md")
		f, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("creating SKILL.md: %w", err)
		}
		defer f.Close()

		data := skill.TemplateData{
			Binary:   skillBinary,
			PlansDir: skillPlansDir,
			Plans:    plans,
		}
		if err := skill.Render(f, data); err != nil {
			return fmt.Errorf("rendering template: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Skill generated: %s\n", outPath)
		return nil
	},
}

func init() {
	skillCmd.Flags().StringVar(&skillPlansDir, "plans", "", "Directory containing plan YAML files")
	skillCmd.Flags().StringVar(&skillOutput, "output", ".claude/skills/declaragent", "Output directory for SKILL.md")
	skillCmd.Flags().StringVar(&skillBinary, "binary", "declaragent", "Binary name to use in generated instructions")
	rootCmd.AddCommand(skillCmd)
}
