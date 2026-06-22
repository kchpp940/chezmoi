package cmd

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strconv"
	"testing"

	"github.com/alecthomas/assert/v2"
	"github.com/spf13/cobra"
	"github.com/twpayne/go-vfs/v5"
	"github.com/twpayne/go-xdg/v6"

	"chezmoi.io/chezmoi/v2/internal/chezmoi"
	"chezmoi.io/chezmoi/v2/internal/chezmoiset"
	"chezmoi.io/chezmoi/v2/internal/chezmoitest"
)

func TestConfigFileFieldTagNamesMatch(t *testing.T) {
	expectedTags := []string{"json", "mapstructure", "yaml"}
	for _, field := range reflect.VisibleFields(reflect.TypeFor[ConfigFile]()) {
		t.Run(field.Name, func(t *testing.T) {
			tagValues := chezmoiset.New[string]()
			for _, tagName := range expectedTags {
				tagValue, ok := field.Tag.Lookup(tagName)
				assert.True(t, ok, "missing %s tag", tagName)
				tagValues.Add(tagValue)
			}
			elements := slices.Sorted(tagValues.Elements())
			assert.Equal(t, []string{elements[0]}, elements, "inconsistent tag values")
		})
	}
}

func TestAddTemplateFuncPanic(t *testing.T) {
	chezmoitest.WithTestFS(t, nil, func(fileSystem vfs.FS) {
		config := newTestConfig(t, fileSystem)
		assert.NotPanics(t, func() {
			config.addTemplateFunc("func", nil)
		})
		assert.Panics(t, func() {
			config.addTemplateFunc("func", nil)
		})
	})
}

func TestConfigFileFormatRoundTrip(t *testing.T) {
	for _, format := range []chezmoi.Format{
		chezmoi.FormatJSON,
		chezmoi.FormatYAML,
	} {
		t.Run(format.Name(), func(t *testing.T) {
			configFile := ConfigFile{
				Color:        autoBool{auto: true},
				Data:         map[string]any{},
				Env:          map[string]string{},
				Hooks:        map[string]hookConfig{},
				Interpreters: map[string]chezmoi.Interpreter{},
				Mode:         chezmoi.ModeFile,
				PINEntry: pinEntryConfig{
					Args:    []string{},
					Options: []string{},
				},
				ScriptEnv: map[string]string{},
				Template: templateConfig{
					Options: []string{},
				},
				TextConv:      []*textConvElement{},
				UseBuiltinAge: autoBool{value: false},
				UseBuiltinGit: autoBool{value: true},
				Dashlane: dashlaneConfig{
					Args: []string{},
				},
				Doppler: dopplerConfig{
					Args: []string{},
				},
				Keepassxc: keepassxcConfig{
					Args: []string{},
				},
				Keeper: keeperConfig{
					Args: []string{},
				},
				Passhole: passholeConfig{
					Args: []string{},
				},
				Secret: secretConfig{
					Args: []string{},
				},
				Age: chezmoi.AgeEncryption{
					Args:            []string{},
					Identity:        chezmoi.NewAbsPath("/identity.txt"),
					Identities:      []chezmoi.AbsPath{},
					Recipients:      []string{},
					RecipientsFiles: []chezmoi.AbsPath{},
				},
				GPG: chezmoi.GPGEncryption{
					Args:       []string{},
					Recipients: []string{},
				},
				Add: addCmdConfig{
					Secrets: newChoiceFlag(severityWarning, nil),
				},
				CD: cdCmdConfig{
					Args: []string{},
				},
				Diff: diffCmdConfig{
					Args: []string{},
				},
				Edit: editCmdConfig{
					Args: []string{},
				},
				Merge: mergeCmdConfig{
					Args: []string{},
				},
				Update: updateCmdConfig{
					Args: []string{},
				},
			}
			data, err := format.Marshal(configFile)
			assert.NoError(t, err)
			var actualConfigFile ConfigFile
			assert.NoError(t, format.Unmarshal(data, &actualConfigFile))
			assert.Equal(t, configFile, actualConfigFile)
		})
	}
}

func TestParseCommand(t *testing.T) {
	for i, tc := range []struct {
		command         string
		args            []string
		expectedCommand string
		expectedArgs    []string
		expectedErr     bool
	}{
		{
			command:         "chezmoi-editor",
			expectedCommand: "chezmoi-editor",
		},
		{
			command:         `chezmoi-editor -f --nomru -c "au VimLeave * !open -a Terminal"`,
			expectedCommand: "chezmoi-editor",
			expectedArgs:    []string{"-f", "--nomru", "-c", "au VimLeave * !open -a Terminal"},
		},
		{
			command:         `"chezmoi editor" $CHEZMOI_TEST_VAR`,
			args:            []string{"extra-arg"},
			expectedCommand: "chezmoi editor",
			expectedArgs:    []string{"chezmoi-test-value", "extra-arg"},
		},
		{
			command:     `"chezmoi editor`,
			expectedErr: true,
		},
	} {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Setenv("CHEZMOI_TEST_VAR", "chezmoi-test-value")
			actualCommand, actualArgs, err := parseCommand(tc.command, tc.args)
			if tc.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedCommand, actualCommand)
				assert.Equal(t, tc.expectedArgs, actualArgs)
			}
		})
	}
}

func TestParseConfig(t *testing.T) {
	for _, tc := range []struct {
		name          string
		filename      string
		contents      string
		expectedColor bool
	}{
		{
			name:     "json_bool",
			filename: "chezmoi.json",
			contents: chezmoitest.JoinLines(
				`{`,
				`  "color":true`,
				`}`,
			),
			expectedColor: true,
		},
		{
			name:     "json_string",
			filename: "chezmoi.json",
			contents: chezmoitest.JoinLines(
				`{`,
				`  "color":"on"`,
				`}`,
			),
			expectedColor: true,
		},
		{
			name:     "toml_bool",
			filename: "chezmoi.toml",
			contents: chezmoitest.JoinLines(
				`color = true`,
			),
			expectedColor: true,
		},
		{
			name:     "toml_string",
			filename: "chezmoi.toml",
			contents: chezmoitest.JoinLines(
				`color = "y"`,
			),
			expectedColor: true,
		},
		{
			name:     "yaml_bool",
			filename: "chezmoi.yaml",
			contents: chezmoitest.JoinLines(
				`color: true`,
			),
			expectedColor: true,
		},
		{
			name:     "yaml_string",
			filename: "chezmoi.yaml",
			contents: chezmoitest.JoinLines(
				`color: "yes"`,
			),
			expectedColor: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			chezmoitest.WithTestFS(t, map[string]any{
				"/home/user/.config/chezmoi/" + tc.filename: tc.contents,
			}, func(fileSystem vfs.FS) {
				c := newTestConfig(t, fileSystem)
				assert.NoError(t, c.execute([]string{"init"}))
				assert.Equal(t, tc.expectedColor, c.Color.Value(c.colorAutoFunc))
			})
		})
	}
}

func TestPrependParentRelPaths(t *testing.T) {
	for _, tc := range []struct {
		name                string
		relPathStrs         []string
		expectedRelPathStrs []string
	}{
		{
			name: "empty",
		},
		{
			name:                "single",
			relPathStrs:         []string{"a"},
			expectedRelPathStrs: []string{"a"},
		},
		{
			name:                "multiple",
			relPathStrs:         []string{"a", "b", "c"},
			expectedRelPathStrs: []string{"a", "b", "c"},
		},
		{
			name:                "single_parent",
			relPathStrs:         []string{"a/b"},
			expectedRelPathStrs: []string{"a", "a/b"},
		},
		{
			name:                "multiple_parents",
			relPathStrs:         []string{"a/b/c"},
			expectedRelPathStrs: []string{"a", "a/b", "a/b/c"},
		},
		{
			name:                "duplicate_parents",
			relPathStrs:         []string{"a/b/c", "a/b/d"},
			expectedRelPathStrs: []string{"a", "a/b", "a/b/c", "a/b/d"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			relPaths := make([]chezmoi.RelPath, len(tc.relPathStrs))
			for i, relPathStr := range tc.relPathStrs {
				relPaths[i] = chezmoi.NewRelPath(relPathStr)
			}
			expected := make([]chezmoi.RelPath, len(tc.expectedRelPathStrs))
			for i, relPathStr := range tc.expectedRelPathStrs {
				expected[i] = chezmoi.NewRelPath(relPathStr)
			}
			assert.Equal(t, expected, prependParentRelPaths(relPaths))
		})
	}
}

func TestInitConfigWithIncludedTemplate(t *testing.T) {
	mainFilename := ".chezmoi.yaml.tmpl"
	secondaryFilename := "personal.config.yaml.tmpl"
	mainContents := chezmoitest.JoinLines(
		`color: true`,
		fmt.Sprintf(`{{ includeTemplate %q . }}`, secondaryFilename),
	)
	secondaryContents := chezmoitest.JoinLines(
		`verbose: true`,
		`safe: {{ stdinIsATTY }}`,
	)

	chezmoitest.WithTestFS(t, map[string]any{
		"/home/user/.local/share/chezmoi/" + mainFilename:      mainContents,
		"/home/user/.local/share/chezmoi/" + secondaryFilename: secondaryContents,
	}, func(fileSystem vfs.FS) {
		c := newTestConfig(t, fileSystem)
		assert.NoError(t, c.execute([]string{"init"}))
		assert.True(t, c.Color.Value(c.colorAutoFunc))
		assert.True(t, c.Verbose)
		assert.False(t, c.Safe)
	})
}

func TestUpperSnakeCaseToCamelCase(t *testing.T) {
	for s, expected := range map[string]string{
		"BUG_REPORT_URL":   "bugReportURL",
		"ID":               "id",
		"ID_LIKE":          "idLike",
		"NAME":             "name",
		"VERSION_CODENAME": "versionCodename",
		"VERSION_ID":       "versionID",
	} {
		assert.Equal(t, expected, upperSnakeCaseToCamelCase(s))
	}
}

func TestIssue3980(t *testing.T) {
	tests := []struct {
		name                   string
		filename               string
		contents               string
		expectsErr             bool
		warns                  []string
		expectedEncryptionType any
		usesBuiltinAge         bool
	}{
		{
			name:     "empty_config",
			filename: "chezmoi.toml",
			contents: chezmoitest.JoinLines(
				"",
			),
			expectsErr:             false,
			warns:                  nil,
			expectedEncryptionType: chezmoi.NoEncryption{},
			usesBuiltinAge:         false,
		},
		{
			name:     "empty_encryption_no_configs",
			filename: "chezmoi.toml",
			contents: chezmoitest.JoinLines(
				"encryption = \"\"",
			),
			expectsErr:             false,
			warns:                  nil,
			expectedEncryptionType: chezmoi.NoEncryption{},
			usesBuiltinAge:         false,
		},
		{
			name:     "valid_age_config",
			filename: "chezmoi.toml",
			contents: chezmoitest.JoinLines(
				"encryption = \"age\"",
				"[age]",
				"command = \"fakeage\"",
				"identity = \"key.txt\"",
			),
			expectsErr:             false,
			warns:                  nil,
			expectedEncryptionType: &chezmoi.AgeEncryption{},
			usesBuiltinAge:         true,
		},
		{
			name:     "valid_age_config_no_builtin",
			filename: "chezmoi.toml",
			contents: chezmoitest.JoinLines(
				"encryption = \"age\"",
				"useBuiltinAge = \"off\"",
				"[age]",
				"command = \"fakeage\"",
				"identity = \"key.txt\"",
			),
			expectsErr:             false,
			warns:                  nil,
			expectedEncryptionType: &chezmoi.AgeEncryption{},
			usesBuiltinAge:         false,
		},
		{
			name:     "valid_gpg_config",
			filename: "chezmoi.toml",
			contents: chezmoitest.JoinLines(
				"encryption = \"gpg\"",
				"[gpg]",
				"command = \"fakegpg\"",
				"symmetric = true",
			),
			expectsErr:             false,
			warns:                  nil,
			expectedEncryptionType: &chezmoi.GPGEncryption{},
			usesBuiltinAge:         false,
		},
		{
			name:     "unset_encryption_uses_gpg",
			filename: "chezmoi.toml",
			contents: chezmoitest.JoinLines(
				"[gpg]",
				"command = \"fakegpg\"",
				"symmetric = true",
			),
			expectsErr:             false,
			warns:                  []string{"warning: 'encryption' not set, using gpg configuration"},
			expectedEncryptionType: &chezmoi.GPGEncryption{},
			usesBuiltinAge:         false,
		},
		{
			name:     "unset_encryption_uses_age",
			filename: "chezmoi.toml",
			contents: chezmoitest.JoinLines(
				"[age]",
				"command = \"fakeage\"",
				"identity = \"key.txt\"",
			),
			expectsErr:             false,
			warns:                  []string{"warning: 'encryption' not set, using age configuration"},
			expectedEncryptionType: &chezmoi.AgeEncryption{},
			usesBuiltinAge:         true,
		},
		{
			name:     "unset_encryption_uses_age_no_builtin",
			filename: "chezmoi.toml",
			contents: chezmoitest.JoinLines(
				"useBuiltinAge = \"off\"",
				"[age]",
				"command = \"fakeage\"",
				"identity = \"key.txt\"",
			),
			expectsErr:             false,
			warns:                  []string{"warning: 'encryption' not set, using age configuration"},
			expectedEncryptionType: &chezmoi.AgeEncryption{},
			usesBuiltinAge:         false,
		},
		{
			name:     "unset_encryption_gpg_priority",
			filename: "chezmoi.toml",
			contents: chezmoitest.JoinLines(
				"[age]",
				"command = \"fakeage\"",
				"identity = \"key.txt\"",
				"[gpg]",
				"command = \"fakegpg\"",
				"symmetric = true",
			),
			expectsErr:             false,
			warns:                  []string{"warning: 'encryption' not set, using gpg configuration"},
			expectedEncryptionType: &chezmoi.GPGEncryption{},
			usesBuiltinAge:         false,
		},
		{
			name:     "unknown_encryption",
			filename: "chezmoi.toml",
			contents: chezmoitest.JoinLines(
				"encryption = \"unknown\"",
			),
			expectsErr:     true,
			warns:          nil,
			usesBuiltinAge: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chezmoitest.WithTestFS(t, map[string]any{
				"/home/user/.config/chezmoi/" + tt.filename: tt.contents,
			}, func(fileSystem vfs.FS) {
				c := newTestConfig(t, fileSystem)

				// Create a buffer to capture stderr.
				var stderr bytes.Buffer
				c.stderr = &stderr

				err := c.execute([]string{"init"})

				if tt.expectsErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
				assert.Equal(t, reflect.TypeOf(tt.expectedEncryptionType), reflect.TypeOf(c.encryption))
				assert.Equal(t, tt.usesBuiltinAge, c.Age.UseBuiltin)

				for _, warn := range tt.warns {
					assert.Contains(t, stderr.String(), warn)
				}
			})
		})
	}
}

func newTestConfig(t *testing.T, fileSystem vfs.FS, options ...configOption) *Config {
	t.Helper()
	system := chezmoi.NewRealSystem(fileSystem)
	config, err := newConfig(
		append([]configOption{
			withBaseSystem(system),
			withDestSystem(system),
			withSourceSystem(system),
			withTestFS(fileSystem),
			withTestUser(t, "user"),
			withUmask(chezmoitest.Umask),
			withVersionInfo(VersionInfo{
				Version: "2.0.0",
			}),
		}, options...)...,
	)
	assert.NoError(t, err)
	return config
}

func withBaseSystem(baseSystem chezmoi.System) configOption {
	return func(c *Config) error {
		c.baseSystem = baseSystem
		return nil
	}
}

func withDestSystem(destSystem chezmoi.System) configOption {
	return func(c *Config) error {
		c.destSystem = destSystem
		return nil
	}
}

func withNoTTY(noTTY bool) configOption { //nolint:unparam
	return func(c *Config) error {
		c.noTTY = noTTY
		return nil
	}
}

func withSourceSystem(sourceSystem chezmoi.System) configOption {
	return func(c *Config) error {
		c.sourceSystem = sourceSystem
		return nil
	}
}

func withStdin(stdin io.Reader) configOption {
	return func(c *Config) error {
		c.stdin = stdin
		return nil
	}
}

func withStdout(stdout io.Writer) configOption {
	return func(c *Config) error {
		c.stdout = stdout
		return nil
	}
}

func withTestFS(fileSystem vfs.FS) configOption {
	return func(c *Config) error {
		c.fileSystem = fileSystem
		return nil
	}
}

func withTestUser(t *testing.T, username string) configOption {
	t.Helper()
	return func(config *Config) error {
		var env string
		switch runtime.GOOS {
		case "plan9":
			config.homeDir = "/home/" + username
			env = "home"
		case "windows":
			config.homeDir = "C:\\home\\" + username
			env = "USERPROFILE"
		default:
			config.homeDir = "/home/" + username
			env = "HOME"
		}
		t.Setenv(env, config.homeDir)
		var err error
		config.homeDirAbsPath, err = chezmoi.NormalizePath(config.homeDir)
		if err != nil {
			t.Fatal(err)
		}
		config.CacheDirAbsPath = config.homeDirAbsPath.JoinString(".cache", "chezmoi")
		config.SourceDirAbsPath = config.homeDirAbsPath.JoinString(".local", "share", "chezmoi")
		config.DestDirAbsPath = config.homeDirAbsPath
		config.Umask = 0o22
		configHome := filepath.Join(config.homeDir, ".config")
		dataHome := filepath.Join(config.homeDir, ".local", "share")
		config.bds = &xdg.BaseDirectorySpecification{
			ConfigHome: configHome,
			ConfigDirs: []string{configHome},
			DataHome:   dataHome,
			DataDirs:   []string{dataHome},
			CacheHome:  filepath.Join(config.homeDir, ".cache"),
			RuntimeDir: filepath.Join(config.homeDir, ".run"),
		}
		return nil
	}
}

func withUmask(umask fs.FileMode) configOption {
	return func(c *Config) error {
		c.Umask = umask
		return nil
	}
}

func TestProfileConfig(t *testing.T) {
	testCases := []struct {
		name            string
		configFile      string
		args            []string
		envProfile      string
		expectedErr     string
		expectedProfile string
		check           func(t *testing.T, c *Config)
	}{
		{
			name: "no_profile",
			configFile: chezmoitest.JoinLines(
				`[profiles.dev]`,
				`  data = { machineType = "dev" }`,
			),
			expectedProfile: "",
		},
		{
			name: "current_profile_in_config",
			configFile: chezmoitest.JoinLines(
				`currentProfile = "dev"`,
				`[profiles.dev]`,
				`  data = { machineType = "dev" }`,
			),
			expectedProfile: "dev",
		},
		{
			name: "profile_from_env",
			configFile: chezmoitest.JoinLines(
				`[profiles.dev]`,
				`  data = { machineType = "dev" }`,
				`[profiles.gpu]`,
				`  data = { machineType = "gpu" }`,
			),
			envProfile:      "gpu",
			expectedProfile: "gpu",
		},
		{
			name: "profile_from_args",
			configFile: chezmoitest.JoinLines(
				`currentProfile = "dev"`,
				`[profiles.dev]`,
				`  data = { machineType = "dev" }`,
				`[profiles.ci]`,
				`  data = { machineType = "ci" }`,
			),
			args:            []string{"--profile", "ci"},
			envProfile:      "gpu",
			expectedProfile: "ci",
		},
		{
			name: "unknown_profile",
			configFile: chezmoitest.JoinLines(
				`[profiles.dev]`,
				`  data = { machineType = "dev" }`,
			),
			args:        []string{"--profile", "unknown"},
			expectedErr: `profile "unknown" not found`,
		},
		{
			name: "profile_data_override",
			configFile: chezmoitest.JoinLines(
				`currentProfile = "dev"`,
				`[data]`,
				`  globalKey = "globalValue"`,
				`  sharedKey = "globalShared"`,
				`[profiles.dev]`,
				`  data = { machineType = "dev", sharedKey = "devShared" }`,
			),
			expectedProfile: "dev",
			check: func(t *testing.T, c *Config) {
				assert.Equal(t, "globalValue", c.Data["globalKey"])
				assert.Equal(t, "devShared", c.Data["sharedKey"])
				assert.Equal(t, "dev", c.Data["machineType"])
			},
		},
		{
			name: "profile_source_dir_override",
			configFile: chezmoitest.JoinLines(
				`currentProfile = "gpu"`,
				`[profiles.gpu]`,
				`  sourceDir = "/home/user/.local/share/chezmoi-gpu"`,
			),
			expectedProfile: "gpu",
			check: func(t *testing.T, c *Config) {
				assert.Equal(t, "/home/user/.local/share/chezmoi-gpu", c.SourceDirAbsPath.String())
			},
		},
		{
			name: "profile_refresh_externals_override",
			configFile: chezmoitest.JoinLines(
				`currentProfile = "ci"`,
				`[profiles.ci]`,
				`  refreshExternals = "always"`,
			),
			expectedProfile: "ci",
			check: func(t *testing.T, c *Config) {
				assert.Equal(t, chezmoi.RefreshExternalsAlways, c.refreshExternals)
			},
		},
		{
			name: "profile_apply_options",
			configFile: chezmoitest.JoinLines(
				`currentProfile = "dev"`,
				`[profiles.dev.apply]`,
				`  exclude = ["scripts"]`,
			),
			expectedProfile: "dev",
			check: func(t *testing.T, c *Config) {
				assert.True(t, c.Apply.Exclude.Bits()&chezmoi.EntryTypeScripts != 0)
			},
		},
		{
			name: "profile_script_condition",
			configFile: chezmoitest.JoinLines(
				`currentProfile = "ci"`,
				`[profiles.ci]`,
				`  scriptCondition = "always"`,
			),
			expectedProfile: "ci",
			check: func(t *testing.T, c *Config) {
				assert.True(t, c.apply.filter.Include.Bits()&chezmoi.EntryTypeScripts != 0)
				assert.True(t, c.apply.filter.Include.Bits()&chezmoi.EntryTypeAlways != 0)
			},
		},
		{
			name: "profile_env_override",
			configFile: chezmoitest.JoinLines(
				`currentProfile = "dev"`,
				`[env]`,
				`  GLOBAL_VAR = "global"`,
				`[profiles.dev]`,
				`  [profiles.dev.env]`,
				`    PROFILE_VAR = "dev"`,
			),
			expectedProfile: "dev",
			check: func(t *testing.T, c *Config) {
				assert.Equal(t, "global", c.Env["GLOBAL_VAR"])
				assert.Equal(t, "dev", c.Env["PROFILE_VAR"])
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envProfile != "" {
				t.Setenv("CHEZMOI_PROFILE", tc.envProfile)
			}

			chezmoitest.WithTestFS(t, map[string]any{
				"/home/user/.config/chezmoi/chezmoi.toml": tc.configFile,
			}, func(fileSystem vfs.FS) {
				c := newTestConfig(t, fileSystem)
				if tc.args != nil {
					c.currentProfile = ""
					for i := 0; i < len(tc.args); i += 2 {
						if tc.args[i] == "--profile" && i+1 < len(tc.args) {
							c.currentProfile = tc.args[i+1]
						}
					}
				}
				configFileAbsPath := chezmoi.NewAbsPath("/home/user/.config/chezmoi/chezmoi.toml")
				err := c.readConfig(configFileAbsPath)
				assert.NoError(t, err)

				err = c.applyProfile()
				if tc.expectedErr != "" {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), tc.expectedErr)
					return
				}
				assert.NoError(t, err)

				assert.Equal(t, tc.expectedProfile, c.getSelectedProfile())
				if tc.check != nil {
					tc.check(t, c)
				}
			})
		})
	}
}

func TestProfileIsolation(t *testing.T) {
	configContent := chezmoitest.JoinLines(
		`[profiles.dev]`,
		`  data = { machineType = "dev", isGPU = false }`,
		`  sourceDir = "/home/user/.local/share/chezmoi-dev"`,
		`  refreshExternals = "never"`,
		`[profiles.gpu]`,
		`  data = { machineType = "gpu", isGPU = true }`,
		`  sourceDir = "/home/user/.local/share/chezmoi-gpu"`,
		`  refreshExternals = "always"`,
	)

	chezmoitest.WithTestFS(t, map[string]any{
		"/home/user/.config/chezmoi/chezmoi.toml": configContent,
	}, func(fileSystem vfs.FS) {
		configFileAbsPath := chezmoi.NewAbsPath("/home/user/.config/chezmoi/chezmoi.toml")

		c1 := newTestConfig(t, fileSystem)
		c1.currentProfile = "dev"
		assert.NoError(t, c1.readConfig(configFileAbsPath))
		assert.NoError(t, c1.applyProfile())
		assert.Equal(t, "dev", c1.getSelectedProfile())
		assert.Equal(t, "dev", c1.Data["machineType"])
		assert.Equal(t, false, c1.Data["isGPU"])
		assert.Equal(t, "/home/user/.local/share/chezmoi-dev", c1.SourceDirAbsPath.String())
		assert.Equal(t, chezmoi.RefreshExternalsNever, c1.refreshExternals)

		c2 := newTestConfig(t, fileSystem)
		c2.currentProfile = "gpu"
		assert.NoError(t, c2.readConfig(configFileAbsPath))
		assert.NoError(t, c2.applyProfile())
		assert.Equal(t, "gpu", c2.getSelectedProfile())
		assert.Equal(t, "gpu", c2.Data["machineType"])
		assert.Equal(t, true, c2.Data["isGPU"])
		assert.Equal(t, "/home/user/.local/share/chezmoi-gpu", c2.SourceDirAbsPath.String())
		assert.Equal(t, chezmoi.RefreshExternalsAlways, c2.refreshExternals)

		assert.NotEqual(t, c1.Data["machineType"], c2.Data["machineType"])
		assert.NotEqual(t, c1.SourceDirAbsPath, c2.SourceDirAbsPath)
		assert.NotEqual(t, c1.refreshExternals, c2.refreshExternals)
	})
}

func TestProfileStateIsolation(t *testing.T) {
	configContent := chezmoitest.JoinLines(
		`[profiles.dev]`,
		`  data = { machineType = "dev" }`,
		`[profiles.gpu]`,
		`  data = { machineType = "gpu" }`,
	)

	chezmoitest.WithTestFS(t, map[string]any{
		"/home/user/.config/chezmoi/chezmoi.toml": configContent,
	}, func(fileSystem vfs.FS) {
		configFileAbsPath := chezmoi.NewAbsPath("/home/user/.config/chezmoi/chezmoi.toml")

		c1 := newTestConfig(t, fileSystem)
		c1.currentProfile = "dev"
		assert.NoError(t, c1.readConfig(configFileAbsPath))
		assert.NoError(t, c1.applyProfile())
		assert.Equal(t, "/home/user/.config/chezmoi/profiles/dev/chezmoistate.boltdb", c1.PersistentStateAbsPath.String())
		assert.Contains(t, c1.CacheDirAbsPath.String(), "profiles/dev")

		c2 := newTestConfig(t, fileSystem)
		c2.currentProfile = "gpu"
		assert.NoError(t, c2.readConfig(configFileAbsPath))
		assert.NoError(t, c2.applyProfile())
		assert.Equal(t, "/home/user/.config/chezmoi/profiles/gpu/chezmoistate.boltdb", c2.PersistentStateAbsPath.String())
		assert.Contains(t, c2.CacheDirAbsPath.String(), "profiles/gpu")

		assert.NotEqual(t, c1.PersistentStateAbsPath, c2.PersistentStateAbsPath)
		assert.NotEqual(t, c1.CacheDirAbsPath, c2.CacheDirAbsPath)
	})
}

func TestProfileExplicitStatePaths(t *testing.T) {
	configContent := chezmoitest.JoinLines(
		`[profiles.dev]`,
		`  persistentState = "/custom/state/dev.boltdb"`,
		`  cacheDir = "/custom/cache/dev"`,
		`[profiles.gpu]`,
		`  data = { machineType = "gpu" }`,
	)

	chezmoitest.WithTestFS(t, map[string]any{
		"/home/user/.config/chezmoi/chezmoi.toml": configContent,
	}, func(fileSystem vfs.FS) {
		configFileAbsPath := chezmoi.NewAbsPath("/home/user/.config/chezmoi/chezmoi.toml")

		c1 := newTestConfig(t, fileSystem)
		c1.currentProfile = "dev"
		assert.NoError(t, c1.readConfig(configFileAbsPath))
		assert.NoError(t, c1.applyProfile())
		assert.Equal(t, "/custom/state/dev.boltdb", c1.PersistentStateAbsPath.String())
		assert.Equal(t, "/custom/cache/dev", c1.CacheDirAbsPath.String())

		c2 := newTestConfig(t, fileSystem)
		c2.currentProfile = "gpu"
		assert.NoError(t, c2.readConfig(configFileAbsPath))
		assert.NoError(t, c2.applyProfile())
		assert.Equal(t, "/home/user/.config/chezmoi/profiles/gpu/chezmoistate.boltdb", c2.PersistentStateAbsPath.String())
		assert.Contains(t, c2.CacheDirAbsPath.String(), "profiles/gpu")
	})
}

func TestProfileNoProfileDefaultPaths(t *testing.T) {
	configContent := chezmoitest.JoinLines(
		`[profiles.dev]`,
		`  data = { machineType = "dev" }`,
	)

	chezmoitest.WithTestFS(t, map[string]any{
		"/home/user/.config/chezmoi/chezmoi.toml": configContent,
	}, func(fileSystem vfs.FS) {
		configFileAbsPath := chezmoi.NewAbsPath("/home/user/.config/chezmoi/chezmoi.toml")

		c := newTestConfig(t, fileSystem)
		assert.NoError(t, c.readConfig(configFileAbsPath))
		assert.NoError(t, c.applyProfile())
		assert.True(t, c.PersistentStateAbsPath.IsEmpty())
		assert.NotContains(t, c.CacheDirAbsPath.String(), "profiles")
	})
}

func TestProfileTemplateData(t *testing.T) {
	configContent := chezmoitest.JoinLines(
		`currentProfile = "dev"`,
		`[profiles.dev]`,
		`  data = { machineType = "dev" }`,
	)

	chezmoitest.WithTestFS(t, map[string]any{
		"/home/user/.config/chezmoi/chezmoi.toml": configContent,
	}, func(fileSystem vfs.FS) {
		c := newTestConfig(t, fileSystem)
		configFileAbsPath := chezmoi.NewAbsPath("/home/user/.config/chezmoi/chezmoi.toml")
		assert.NoError(t, c.readConfig(configFileAbsPath))
		assert.NoError(t, c.applyProfile())

		rootCmd, err := c.newRootCmd()
		assert.NoError(t, err)

		templateDataMap := c.getTemplateDataMap(rootCmd)
		chezmoiData, ok := templateDataMap["chezmoi"].(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "dev", chezmoiData["profile"])
	})
}

func TestProfileFlagCompletion(t *testing.T) {
	configContent := chezmoitest.JoinLines(
		`[profiles.alpha]`,
		`  data = { name = "alpha" }`,
		`[profiles.beta]`,
		`  data = { name = "beta" }`,
		`[profiles.gamma]`,
		`  data = { name = "gamma" }`,
	)

	chezmoitest.WithTestFS(t, map[string]any{
		"/home/user/.config/chezmoi/chezmoi.toml": configContent,
	}, func(fileSystem vfs.FS) {
		c := newTestConfig(t, fileSystem)
		configFileAbsPath := chezmoi.NewAbsPath("/home/user/.config/chezmoi/chezmoi.toml")
		assert.NoError(t, c.readConfig(configFileAbsPath))

		rootCmd, err := c.newRootCmd()
		assert.NoError(t, err)

		completions, directive := c.profileFlagCompletionFunc(rootCmd, nil, "")
		assert.Equal(t, []string{"alpha", "beta", "gamma"}, completions)
		assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)

		completions, _ = c.profileFlagCompletionFunc(rootCmd, nil, "b")
		assert.Equal(t, []string{"beta"}, completions)
	})
}
