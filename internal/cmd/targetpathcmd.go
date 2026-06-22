package cmd

import (
	"strings"

	"github.com/spf13/cobra"

	"chezmoi.io/chezmoi/v2/internal/chezmoi"
)

func (c *Config) newTargetPathCmd() *cobra.Command {
	targetPathCmd := &cobra.Command{
		GroupID: groupIDInternal,
		Use:     "target-path [source-path]...",
		Short:   "Print the target path of a source path",
		Long:    mustLongHelp("target-path"),
		Example: example("target-path"),
		RunE:    c.runTargetPathCmd,
		Annotations: newAnnotations(
			persistentStateModeReadMockWrite,
		),
	}

	return targetPathCmd
}

func (c *Config) runTargetPathCmd(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return c.writeOutputString(c.DestDirAbsPath.String()+"\n", 0o666)
	}

	builder := strings.Builder{}
	sourceDirAbsPath, err := c.getSourceDirAbsPath(nil)
	if err != nil {
		return err
	}

	for _, arg := range args {
		argAbsPath, err := chezmoi.NewAbsPathFromExtPath(arg, c.homeDirAbsPath)
		if err != nil {
			return err
		}

		targetRelPath, err := chezmoi.SourceAbsPathToTargetRelPath(
			c.sourceSystem,
			sourceDirAbsPath,
			argAbsPath,
			c.encryption.EncryptedSuffix(),
		)
		if err != nil {
			return err
		}

		if _, err := builder.WriteString(c.DestDirAbsPath.String()); err != nil {
			return err
		}
		if err := builder.WriteByte('/'); err != nil {
			return err
		}
		if _, err := builder.WriteString(targetRelPath.String()); err != nil {
			return err
		}
		if err := builder.WriteByte('\n'); err != nil {
			return err
		}
	}

	return c.writeOutputString(builder.String(), 0o666)
}
