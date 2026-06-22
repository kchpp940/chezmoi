package cmd

import (
	"fmt"
	"io/fs"
	"log/slog"
	"strings"

	"github.com/spf13/cobra"

	"chezmoi.io/chezmoi/v2/internal/chezmoi"
	"chezmoi.io/chezmoi/v2/internal/chezmoilog"
)

type statusCmdConfig struct {
	Exclude    *chezmoi.EntryTypeSet `json:"exclude"   mapstructure:"exclude"   yaml:"exclude"`
	PathStyle  *choiceFlag           `json:"pathStyle" mapstructure:"pathStyle" yaml:"pathStyle"`
	include    *chezmoi.EntryTypeSet
	init       bool
	parentDirs bool
	recursive  bool
}

func (c *Config) newStatusCmd() *cobra.Command {
	statusCmd := &cobra.Command{
		GroupID:           groupIDDaily,
		Use:               "status [target]...",
		Short:             "Show the status of targets",
		Long:              mustLongHelp("status"),
		Example:           example("status"),
		ValidArgsFunction: c.targetValidArgs,
		RunE:              c.runStatusCmd,
		Annotations: newAnnotations(
			dryRun,
			persistentStateModeReadMockWrite,
			requiresSourceDirectory,
		),
	}

	statusCmd.Flags().VarP(c.Status.Exclude, "exclude", "x", "Exclude entry types")
	statusCmd.Flags().VarP(c.Status.PathStyle, "path-style", "p", "Path style")
	must(statusCmd.RegisterFlagCompletionFunc("path-style", c.Status.PathStyle.FlagCompletionFunc()))
	statusCmd.Flags().VarP(c.Status.include, "include", "i", "Include entry types")
	statusCmd.Flags().BoolVar(&c.Status.init, "init", c.Status.init, "Recreate config file from template")
	statusCmd.Flags().
		BoolVarP(&c.Status.parentDirs, "parent-dirs", "P", c.Status.parentDirs, "Show status of all parent directories")
	statusCmd.Flags().BoolVarP(&c.Status.recursive, "recursive", "r", c.Status.recursive, "Recurse into subdirectories")

	return statusCmd
}

func (c *Config) runStatusCmd(cmd *cobra.Command, args []string) error {
	builder := strings.Builder{}
	preApplyFunc := func(decision chezmoi.StateDecision) error {
		c.logger.Info("statusPreApplyFunc",
			chezmoilog.Stringer("targetRelPath", decision.TargetRelPath),
			slog.Any("targetEntryState", decision.TargetEntryState),
			slog.Any("lastWrittenEntryState", decision.LastWrittenEntryState),
			slog.Any("actualEntryState", decision.ActualEntryState),
		)

		var (
			x = ' '
			y = ' '
		)

		switch {
		case decision.TargetEntryState != nil && decision.TargetEntryState.Type == chezmoi.EntryStateTypeScript:
			y = 'R'
		case decision.IsRegularFile:
			if decision.FromTextConvResult.Err != nil {
				c.errorf("%s: textconv from actual failed: %v\n", decision.TargetRelPath, decision.FromTextConvResult.Err)
			}
			if decision.ToTextConvResult.Err != nil {
				c.errorf("%s: textconv to target failed: %v\n", decision.TargetRelPath, decision.ToTextConvResult.Err)
			}
			if decision.NeedReportDrift {
				x = statusRune(decision.LastWrittenEntryState, decision.ActualEntryState)
				y = statusRune(decision.ActualEntryState, decision.TargetEntryState)
			}
		default:
			if decision.TargetEntryState != nil && !decision.TargetEntryState.Equivalent(decision.ActualEntryState) {
				x = statusRune(decision.LastWrittenEntryState, decision.ActualEntryState)
				y = statusRune(decision.ActualEntryState, decision.TargetEntryState)
			}
		}

		if x != ' ' || y != ' ' {
			var path string
			switch pathStyle := c.Status.PathStyle.String(); pathStyle {
			case pathStyleAbsolute:
				path = c.DestDirAbsPath.Join(decision.TargetRelPath).String()
			case pathStyleRelative:
				path = decision.TargetRelPath.String()
			default:
				return fmt.Errorf("%s: invalid path style", pathStyle)
			}

			fmt.Fprintf(&builder, "%c%c %s\n", x, y, path)
		}
		return fs.SkipDir
	}
	if err := c.applyArgs(cmd.Context(), c.destSystem, c.DestDirAbsPath, args, applyArgsOptions{
		cmd:          cmd,
		filter:       chezmoi.NewEntryTypeFilter(c.Status.include.Bits(), c.Status.Exclude.Bits()),
		init:         c.Status.init,
		parentDirs:   c.Status.parentDirs,
		recursive:    c.Status.recursive,
		umask:        c.Umask,
		preApplyFunc: preApplyFunc,
		textConvFunc: c.TextConv.convert,
	}); err != nil {
		return err
	}
	return c.writeOutputString(builder.String(), 0o666)
}

func statusRune(fromState, toState *chezmoi.EntryState) rune {
	if fromState == nil || fromState.Equivalent(toState) {
		return ' '
	}
	switch toState.Type {
	case chezmoi.EntryStateTypeRemove:
		return 'D'
	case chezmoi.EntryStateTypeDir, chezmoi.EntryStateTypeFile, chezmoi.EntryStateTypeSymlink:
		switch fromState.Type {
		case chezmoi.EntryStateTypeRemove:
			return 'A'
		default:
			return 'M'
		}
	case chezmoi.EntryStateTypeScript:
		return 'R'
	default:
		return '?'
	}
}
