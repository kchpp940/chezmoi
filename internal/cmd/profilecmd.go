package cmd

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"chezmoi.io/chezmoi/v2/internal/chezmoi"
)

type profileCmdConfig struct {
	format *choiceFlag
}

type profileSource int

const (
	profileSourceNone profileSource = iota
	profileSourceCommandLine
	profileSourceEnvironment
	profileSourceConfigFile
)

func (ps profileSource) String() string {
	switch ps {
	case profileSourceCommandLine:
		return "Command line (--profile)"
	case profileSourceEnvironment:
		return "Environment variable (CHEZMOI_PROFILE)"
	case profileSourceConfigFile:
		return "Config file (currentProfile)"
	default:
		return "None"
	}
}

func (c *Config) newProfileCmd() *cobra.Command {
	profileCmd := &cobra.Command{
		GroupID:           groupIDDaily,
		Use:               "profile",
		Short:             "Show current profile and effective configuration",
		Long:              mustLongHelp("profile"),
		Example:           example("profile"),
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE:              c.runProfileShowCmd,
		Annotations: newAnnotations(
			persistentStateModeReadOnly,
		),
	}

	profileCmd.Flags().VarP(c.profileCmd.format, "format", "f", "Output format")
	must(profileCmd.RegisterFlagCompletionFunc("format", c.profileCmd.format.FlagCompletionFunc()))

	profileListCmd := &cobra.Command{
		Use:               "list",
		Short:             "List all available profiles",
		Long:              mustLongHelp("profile-list"),
		Example:           example("profile-list"),
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE:              c.runProfileListCmd,
		Annotations: newAnnotations(
			persistentStateModeReadOnly,
		),
	}
	profileListCmd.Flags().VarP(c.profileCmd.format, "format", "f", "Output format")
	must(profileListCmd.RegisterFlagCompletionFunc("format", c.profileCmd.format.FlagCompletionFunc()))
	profileCmd.AddCommand(profileListCmd)

	return profileCmd
}

func (c *Config) runProfileShowCmd(cmd *cobra.Command, args []string) error {
	ps := c.profileSource
	selectedProfile := c.getSelectedProfile()

	ec, err := c.buildEffectiveConfig()
	if err != nil {
		return err
	}

	format := c.profileCmd.format.String()
	switch format {
	case "json":
		return c.marshal(format, map[string]any{
			"active":          ec.ProfileName,
			"source":          ps.String(),
			"effectiveConfig": ecToMap(ec, c),
			"available":       sortedProfileNames(c),
		})
	case "yaml":
		return c.marshal(format, map[string]any{
			"active":          ec.ProfileName,
			"source":          ps.String(),
			"effectiveConfig": ecToMap(ec, c),
			"available":       sortedProfileNames(c),
		})
	default:
		return writeProfileText(c.stdout, ec, ps, selectedProfile, c)
	}
}

func (c *Config) runProfileListCmd(cmd *cobra.Command, args []string) error {
	selectedProfile := c.getSelectedProfile()
	profileNames := make([]string, 0, len(c.Profiles))
	for name := range c.Profiles {
		profileNames = append(profileNames, name)
	}
	slices.Sort(profileNames)

	format := c.profileCmd.format.String()
	switch format {
	case "json", "yaml":
		return c.marshal(format, map[string]any{
			"profiles": profileNames,
			"active":   selectedProfile,
		})
	default:
		w := c.stdout
		if len(profileNames) == 0 {
			fmt.Fprintln(w, "No profiles configured.")
			return nil
		}
		for _, name := range profileNames {
			if name == selectedProfile {
				fmt.Fprintf(w, "  %s (active)\n", name)
			} else {
				fmt.Fprintf(w, "  %s\n", name)
			}
		}
		return nil
	}
}

func (c *Config) computeProfileSource(cmd *cobra.Command) profileSource {
	if profileFlag := cmd.Root().Flags().Lookup("profile"); profileFlag != nil && profileFlag.Changed {
		return profileSourceCommandLine
	}
	if os.Getenv("CHEZMOI_PROFILE") != "" {
		return profileSourceEnvironment
	}
	if c.CurrentProfile != "" {
		return profileSourceConfigFile
	}
	return profileSourceNone
}

func ecToMap(ec *effectiveConfig, c *Config) map[string]any {
	sourceDir := ec.SourceDirAbsPath.String()
	if ec.SourceDirAbsPath.IsEmpty() {
		if resolved, err := c.getSourceDirAbsPath(nil); err == nil {
			sourceDir = resolved.String()
		}
	}
	persistentState := ec.PersistentStatePath.String()
	if ec.PersistentStatePath.IsEmpty() {
		if resolved, err := c.persistentStateFile(); err == nil {
			persistentState = resolved.String()
		}
	}
	return map[string]any{
		"sourceDir":        sourceDir,
		"cacheDir":         ec.CacheDirAbsPath.String(),
		"persistentState":  persistentState,
		"refreshExternals": ec.RefreshExternals.String(),
		"scriptCondition":  string(ec.ScriptCondition),
		"applyInit":        ec.ApplyInit,
		"applyInclude":     entryTypeSetBits(ec.ApplyFilter.Include),
		"applyExclude":     entryTypeSetBits(ec.ApplyFilter.Exclude),
		"mode":             string(ec.Mode),
		"umask":            fmt.Sprintf("%04o", ec.Umask),
		"data":             ec.Data,
	}
}

func writeProfileText(w io.Writer, ec *effectiveConfig, ps profileSource, selectedProfile string, c *Config) error {
	if selectedProfile == "" {
		fmt.Fprintln(w, "Current Profile: <none>")
	} else {
		fmt.Fprintf(w, "Current Profile: %s\n", selectedProfile)
	}
	fmt.Fprintf(w, "Source:          %s\n", ps.String())
	fmt.Fprintln(w)

	fmt.Fprintln(w, "Effective Configuration:")
	sourceDir := ec.SourceDirAbsPath.String()
	if ec.SourceDirAbsPath.IsEmpty() {
		if resolved, err := c.getSourceDirAbsPath(nil); err == nil {
			sourceDir = resolved.String()
		}
	}
	writeIndentField(w, 2, "Source Dir", sourceDir)
	writeIndentField(w, 2, "Cache Dir", ec.CacheDirAbsPath.String())
	persistentState := ec.PersistentStatePath.String()
	if ec.PersistentStatePath.IsEmpty() {
		if resolved, err := c.persistentStateFile(); err == nil {
			persistentState = resolved.String()
		}
	}
	writeIndentField(w, 2, "Persistent State", persistentState)
	writeIndentField(w, 2, "Refresh Externals", ec.RefreshExternals.String())
	if ec.ScriptCondition != "" {
		writeIndentField(w, 2, "Script Condition", string(ec.ScriptCondition))
	} else {
		writeIndentField(w, 2, "Script Condition", "<none>")
	}
	writeIndentField(w, 2, "Apply Init", fmt.Sprintf("%t", ec.ApplyInit))
	if ec.ApplyFilter.Include != nil {
		writeIndentField(w, 2, "Apply Include", entryTypeSetString(ec.ApplyFilter.Include))
	}
	if ec.ApplyFilter.Exclude != nil {
		writeIndentField(w, 2, "Apply Exclude", entryTypeSetString(ec.ApplyFilter.Exclude))
	}
	writeIndentField(w, 2, "Mode", string(ec.Mode))
	writeIndentField(w, 2, "Umask", fmt.Sprintf("%04o", ec.Umask))

	if selectedProfile != "" {
		if profile, ok := c.Profiles[selectedProfile]; ok {
			fmt.Fprintln(w)
			fmt.Fprintln(w, "Profile Overrides:")
			overrides := getProfileOverrides(profile, c)
			if len(overrides) == 0 {
				fmt.Fprintln(w, "  (no overrides)")
			} else {
				for _, o := range overrides {
					fmt.Fprintf(w, "  %s\n", o)
				}
			}
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Available Profiles:")
	profileNames := make([]string, 0, len(c.Profiles))
	for name := range c.Profiles {
		profileNames = append(profileNames, name)
	}
	slices.Sort(profileNames)
	if len(profileNames) == 0 {
		fmt.Fprintln(w, "  (none)")
	} else {
		for _, name := range profileNames {
			if name == selectedProfile {
				fmt.Fprintf(w, "  * %s\n", name)
			} else {
				fmt.Fprintf(w, "    %s\n", name)
			}
		}
	}

	return nil
}

func writeIndentField(w io.Writer, indent int, key, value string) {
	prefix := strings.Repeat(" ", indent*2)
	fmt.Fprintf(w, "%s%-18s %s\n", prefix, key+":", value)
}

func entryTypeSetString(s *chezmoi.EntryTypeSet) string {
	if s.Bits() == chezmoi.EntryTypesAll {
		return "all"
	}
	if s.Bits() == chezmoi.EntryTypesNone {
		return "none"
	}
	return s.String()
}

func getProfileOverrides(profile profileConfig, c *Config) []string {
	var overrides []string

	if !profile.SourceDirAbsPath.IsEmpty() {
		overrides = append(overrides, fmt.Sprintf("sourceDir = %q", profile.SourceDirAbsPath.String()))
	}
	if !profile.CacheDirAbsPath.IsEmpty() {
		overrides = append(overrides, fmt.Sprintf("cacheDir = %q", profile.CacheDirAbsPath.String()))
	}
	if !profile.PersistentStateAbsPath.IsEmpty() {
		overrides = append(overrides, fmt.Sprintf("persistentState = %q", profile.PersistentStateAbsPath.String()))
	}
	if profile.RefreshExternals != chezmoi.RefreshExternalsAuto {
		overrides = append(overrides, fmt.Sprintf("refreshExternals = %q", profile.RefreshExternals.String()))
	}
	if profile.ScriptCondition != "" {
		overrides = append(overrides, fmt.Sprintf("scriptCondition = %q", string(profile.ScriptCondition)))
	}
	if profile.Mode != "" {
		overrides = append(overrides, fmt.Sprintf("mode = %q", string(profile.Mode)))
	}
	if profile.Umask != 0 {
		overrides = append(overrides, fmt.Sprintf("umask = %04o", profile.Umask))
	}
	if profile.Apply.Init {
		overrides = append(overrides, "apply.init = true")
	}
	if profile.Apply.Include != nil && profile.Apply.Include.Bits() != chezmoi.EntryTypesNone {
		overrides = append(overrides, fmt.Sprintf("apply.include = %s", entryTypeSetString(profile.Apply.Include)))
	}
	if profile.Apply.Exclude != nil && profile.Apply.Exclude.Bits() != chezmoi.EntryTypesNone {
		overrides = append(overrides, fmt.Sprintf("apply.exclude = %s", entryTypeSetString(profile.Apply.Exclude)))
	}
	if len(profile.Data) > 0 {
		for _, k := range sortedProfileDataKeys(profile.Data) {
			v := profile.Data[k]
			overrides = append(overrides, fmt.Sprintf("data.%s = %v", k, formatProfileDataValue(v)))
		}
	}
	if len(profile.Env) > 0 {
		keys := make([]string, 0, len(profile.Env))
		for k := range profile.Env {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			overrides = append(overrides, fmt.Sprintf("env.%s = %q", k, profile.Env[k]))
		}
	}
	if len(profile.ScriptEnv) > 0 {
		keys := make([]string, 0, len(profile.ScriptEnv))
		for k := range profile.ScriptEnv {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			overrides = append(overrides, fmt.Sprintf("scriptEnv.%s = %q", k, profile.ScriptEnv[k]))
		}
	}
	if len(profile.Template.Options) > 0 {
		overrides = append(overrides, fmt.Sprintf("template.options = %v", profile.Template.Options))
	}
	if len(profile.Interpreters) > 0 {
		keys := make([]string, 0, len(profile.Interpreters))
		for k := range profile.Interpreters {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			interp := profile.Interpreters[k]
			overrides = append(overrides, fmt.Sprintf("interpreters.%s = %s %v", k, interp.Command, interp.Args))
		}
	}

	slices.Sort(overrides)
	return overrides
}

func sortedProfileDataKeys(data map[string]any) []string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func formatProfileDataValue(v any) string {
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("%q", val)
	case bool:
		return fmt.Sprintf("%t", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

func sortedProfileNames(c *Config) []string {
	names := make([]string, 0, len(c.Profiles))
	for name := range c.Profiles {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

var entryTypeNames = map[chezmoi.EntryTypeBits]string{
	chezmoi.EntryTypeDirs:      "dirs",
	chezmoi.EntryTypeFiles:     "files",
	chezmoi.EntryTypeRemove:    "remove",
	chezmoi.EntryTypeScripts:   "scripts",
	chezmoi.EntryTypeSymlinks:  "symlinks",
	chezmoi.EntryTypeEncrypted: "encrypted",
	chezmoi.EntryTypeExternals: "externals",
	chezmoi.EntryTypeTemplates: "templates",
	chezmoi.EntryTypeAlways:    "always",
}

func entryTypeSetBits(s *chezmoi.EntryTypeSet) []string {
	if s == nil || s.Bits() == chezmoi.EntryTypesNone {
		return []string{}
	}
	if s.Bits() == chezmoi.EntryTypesAll {
		return []string{"all"}
	}
	bits := s.Bits()
	result := []string{}
	keys := make([]chezmoi.EntryTypeBits, 0, len(entryTypeNames))
	for k := range entryTypeNames {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, bit := range keys {
		if bits&bit != 0 {
			result = append(result, entryTypeNames[bit])
		}
	}
	return result
}
