package cmd

import (
	"github.com/spf13/cobra"

	"chezmoi.io/chezmoi/v2/internal/chezmoi"
)

type applyCmdConfig struct {
	filter     *chezmoi.EntryTypeFilter
	init       bool
	parentDirs bool
	recursive  bool
}

type applyConfigFileConfig struct {
	Exclude *chezmoi.EntryTypeSet `json:"exclude" mapstructure:"exclude" yaml:"exclude"`
	Include *chezmoi.EntryTypeSet `json:"include" mapstructure:"include" yaml:"include"`
	Init    bool                  `json:"init"    mapstructure:"init"    yaml:"init"`
}

func (c *Config) newApplyCmd() *cobra.Command {
	applyCmd := &cobra.Command{
		GroupID:           groupIDDaily,
		Use:               "apply [target]...",
		Short:             "Update the destination directory to match the target state",
		Long:              mustLongHelp("apply"),
		Example:           example("apply"),
		ValidArgsFunction: c.targetValidArgs,
		RunE:              c.runApplyCmd,
		Annotations: newAnnotations(
			modifiesDestinationDirectory,
			persistentStateModeReadWrite,
			requiresSourceDirectory,
		),
	}

	applyCmd.Flags().VarP(c.apply.filter.Exclude, "exclude", "x", "Exclude entry types")
	applyCmd.Flags().VarP(c.apply.filter.Include, "include", "i", "Include entry types")
	applyCmd.Flags().BoolVar(&c.apply.init, "init", c.apply.init, "Recreate config file from template")
	applyCmd.Flags().BoolVarP(&c.apply.parentDirs, "parent-dirs", "P", c.apply.parentDirs, "Apply all parent directories")
	applyCmd.Flags().BoolVarP(&c.apply.recursive, "recursive", "r", c.apply.recursive, "Recurse into subdirectories")

	return applyCmd
}

func (c *Config) runApplyCmd(cmd *cobra.Command, args []string) error {
	filter := c.apply.filter
	if c.profile != nil {
		if c.profile.Apply.Include != nil && c.profile.Apply.Include.Bits() != chezmoi.EntryTypesNone {
			filter.Include = c.profile.Apply.Include
		}
		if c.profile.Apply.Exclude != nil && c.profile.Apply.Exclude.Bits() != chezmoi.EntryTypesNone {
			filter.Exclude = c.profile.Apply.Exclude
		}
	}
	init := c.apply.init
	if c.profile != nil && c.profile.Apply.Init {
		init = c.profile.Apply.Init
	}
	return c.applyArgs(cmd.Context(), c.destSystem, c.DestDirAbsPath, args, applyArgsOptions{
		cmd:          cmd,
		filter:       filter,
		init:         init,
		parentDirs:   c.apply.parentDirs,
		recursive:    c.apply.recursive,
		umask:        c.Umask,
		preApplyFunc: c.defaultPreApplyFunc,
	})
}
