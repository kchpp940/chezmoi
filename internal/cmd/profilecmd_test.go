package cmd

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/alecthomas/assert/v2"
	"github.com/goccy/go-yaml"
	"github.com/twpayne/go-vfs/v5"

	"chezmoi.io/chezmoi/v2/internal/chezmoitest"
)

func TestProfileCmd(t *testing.T) {
	for _, tc := range []struct {
		name         string
		format       string
		unmarshal    func([]byte, any) error
		configFile   string
		extraArgs    []string
		expectActive string
		expectSource string
	}{
		{
			name:   "json_no_profile",
			format: "json",
			configFile: chezmoitest.JoinLines(
				`{`,
				`  "data": {`,
				`    "name": "user"`,
				`  }`,
				`}`,
			),
			unmarshal:    json.Unmarshal,
			expectActive: "",
			expectSource: "None",
		},
		{
			name:   "json_current_profile",
			format: "json",
			configFile: chezmoitest.JoinLines(
				`{`,
				`  "currentProfile": "dev",`,
				`  "profiles": {`,
				`    "dev": {`,
				`      "data": {`,
				`        "machineType": "workstation"`,
				`      },`,
				`      "refreshExternals": "always"`,
				`    },`,
				`    "gpu": {`,
				`      "data": {`,
				`        "machineType": "gpu"`,
				`      }`,
				`    }`,
				`  }`,
				`}`,
			),
			unmarshal:    json.Unmarshal,
			expectActive: "dev",
			expectSource: "Config file (currentProfile)",
		},
		{
			name:   "json_cli_profile",
			format: "json",
			configFile: chezmoitest.JoinLines(
				`{`,
				`  "profiles": {`,
				`    "dev": {`,
				`      "data": {`,
				`        "machineType": "workstation"`,
				`      }`,
				`    },`,
				`    "gpu": {`,
				`      "data": {`,
				`        "machineType": "gpu"`,
				`      }`,
				`    }`,
				`  }`,
				`}`,
			),
			extraArgs:    []string{"--profile=gpu"},
			unmarshal:    json.Unmarshal,
			expectActive: "gpu",
			expectSource: "Command line (--profile)",
		},
		{
			name:   "yaml_current_profile",
			format: "yaml",
			configFile: chezmoitest.JoinLines(
				`currentProfile: dev`,
				`profiles:`,
				`  dev:`,
				`    data:`,
				`      machineType: workstation`,
				`    refreshExternals: always`,
				`  gpu:`,
				`    data:`,
				`      machineType: gpu`,
			),
			unmarshal:    yaml.Unmarshal,
			expectActive: "dev",
			expectSource: "Config file (currentProfile)",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CHEZMOI_PROFILE", "")
			root := map[string]any{
				"home/user/.config/chezmoi/chezmoi.json": tc.configFile,
			}
			if strings.HasPrefix(tc.name, "yaml") {
				root = map[string]any{
					"home/user/.config/chezmoi/chezmoi.yaml": tc.configFile,
				}
			}
			chezmoitest.WithTestFS(t, root, func(fileSystem vfs.FS) {
				args := append(tc.extraArgs,
					"profile",
					"--format", tc.format,
				)
				stdout := strings.Builder{}
				config := newTestConfig(t, fileSystem, withStdout(&stdout))
				assert.NoError(t, config.execute(args))

				var data struct {
					Active           string `json:"active" yaml:"active"`
					Source           string `json:"source" yaml:"source"`
					EffectiveConfig  struct {
						RefreshExternals string `json:"refreshExternals" yaml:"refreshExternals"`
						ApplyInit        bool   `json:"applyInit" yaml:"applyInit"`
					} `json:"effectiveConfig" yaml:"effectiveConfig"`
					Available []string `json:"available" yaml:"available"`
				}
				assert.NoError(t, tc.unmarshal([]byte(stdout.String()), &data))
				assert.Equal(t, tc.expectActive, data.Active)
				assert.Equal(t, tc.expectSource, data.Source)
				if tc.expectActive == "dev" {
					assert.Equal(t, "always", data.EffectiveConfig.RefreshExternals)
					assert.True(t, slices.Contains(data.Available, "dev"))
					assert.True(t, slices.Contains(data.Available, "gpu"))
				}
				if tc.expectActive == "gpu" {
					assert.True(t, slices.Contains(data.Available, "dev"))
					assert.True(t, slices.Contains(data.Available, "gpu"))
				}
			})
		})
	}
}

func TestProfileCmdText(t *testing.T) {
	t.Setenv("CHEZMOI_PROFILE", "")
	configFile := chezmoitest.JoinLines(
		`{`,
		`  "currentProfile": "dev",`,
		`  "profiles": {`,
		`    "dev": {`,
		`      "data": {`,
		`        "machineType": "workstation"`,
		`      },`,
		`      "refreshExternals": "always"`,
		`    },`,
		`    "gpu": {`,
		`      "data": {`,
		`        "machineType": "gpu"`,
		`      }`,
		`    }`,
		`  }`,
		`}`,
	)
	root := map[string]any{
		"home/user/.config/chezmoi/chezmoi.json": configFile,
	}
	chezmoitest.WithTestFS(t, root, func(fileSystem vfs.FS) {
		args := []string{"profile"}
		stdout := strings.Builder{}
		config := newTestConfig(t, fileSystem, withStdout(&stdout))
		assert.NoError(t, config.execute(args))

		output := stdout.String()
		assert.True(t, strings.Contains(output, "Current Profile: dev"), "missing Current Profile, got:\n%s", output)
		assert.True(t, strings.Contains(output, "Source:          Config file (currentProfile)"), "missing Source, got:\n%s", output)
		assert.True(t, strings.Contains(output, "Effective Configuration:"), "missing Effective Configuration, got:\n%s", output)
		assert.True(t, strings.Contains(output, "Refresh Externals"), "missing Refresh Externals, got:\n%s", output)
		assert.True(t, strings.Contains(output, "Profile Overrides:"), "missing Profile Overrides, got:\n%s", output)
		assert.True(t, strings.Contains(output, "data.machineType = \"workstation\""), "missing data override, got:\n%s", output)
		assert.True(t, strings.Contains(output, "refreshExternals = \"always\""), "missing refreshExternals override, got:\n%s", output)
		assert.True(t, strings.Contains(output, "Available Profiles:"), "missing Available Profiles, got:\n%s", output)
		assert.True(t, strings.Contains(output, "* dev"), "missing * dev marker, got:\n%s", output)
		assert.True(t, strings.Contains(output, "    gpu"), "missing gpu entry, got:\n%s", output)
	})
}

func TestProfileListCmd(t *testing.T) {
	for _, tc := range []struct {
		name       string
		format     string
		unmarshal  func([]byte, any) error
		configFile string
		active     string
		profiles   []string
	}{
		{
			name:   "json",
			format: "json",
			configFile: chezmoitest.JoinLines(
				`{`,
				`  "profiles": {`,
				`    "dev": { "data": {} },`,
				`    "gpu": { "data": {} },`,
				`    "ci": { "data": {} }`,
				`  }`,
				`}`,
			),
			unmarshal: json.Unmarshal,
			profiles:  []string{"ci", "dev", "gpu"},
		},
		{
			name:   "yaml",
			format: "yaml",
			configFile: chezmoitest.JoinLines(
				`profiles:`,
				`  dev:`,
				`    data: {}`,
				`  gpu:`,
				`    data: {}`,
				`  ci:`,
				`    data: {}`,
				`currentProfile: ci`,
			),
			unmarshal: yaml.Unmarshal,
			active:    "ci",
			profiles:  []string{"ci", "dev", "gpu"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CHEZMOI_PROFILE", "")
			root := map[string]any{
				"home/user/.config/chezmoi/chezmoi.json": tc.configFile,
			}
			if tc.format == "yaml" {
				root = map[string]any{
					"home/user/.config/chezmoi/chezmoi.yaml": tc.configFile,
				}
			}
			chezmoitest.WithTestFS(t, root, func(fileSystem vfs.FS) {
				args := []string{
					"profile", "list",
					"--format", tc.format,
				}
				stdout := strings.Builder{}
				config := newTestConfig(t, fileSystem, withStdout(&stdout))
				assert.NoError(t, config.execute(args))

				var data struct {
					Profiles []string `json:"profiles" yaml:"profiles"`
					Active   string   `json:"active"   yaml:"active"`
				}
				assert.NoError(t, tc.unmarshal([]byte(stdout.String()), &data))
				assert.Equal(t, tc.profiles, data.Profiles)
				assert.Equal(t, tc.active, data.Active)
			})
		})
	}
}

func TestProfileListCmdText(t *testing.T) {
	t.Setenv("CHEZMOI_PROFILE", "")
	configFile := chezmoitest.JoinLines(
		`{`,
		`  "currentProfile": "dev",`,
		`  "profiles": {`,
		`    "dev": { "data": {} },`,
		`    "gpu": { "data": {} },`,
		`    "ci": { "data": {} }`,
		`  }`,
		`}`,
	)
	root := map[string]any{
		"home/user/.config/chezmoi/chezmoi.json": configFile,
	}
	chezmoitest.WithTestFS(t, root, func(fileSystem vfs.FS) {
		args := []string{"profile", "list"}
		stdout := strings.Builder{}
		config := newTestConfig(t, fileSystem, withStdout(&stdout))
		assert.NoError(t, config.execute(args))

		output := stdout.String()
		assert.True(t, strings.Contains(output, "ci"), "missing ci, got:\n%s", output)
		assert.True(t, strings.Contains(output, "dev (active)"), "missing dev (active), got:\n%s", output)
		assert.True(t, strings.Contains(output, "gpu"), "missing gpu, got:\n%s", output)
	})
}

func TestProfileListCmdEmpty(t *testing.T) {
	t.Setenv("CHEZMOI_PROFILE", "")
	configFile := chezmoitest.JoinLines(
		`{`,
		`  "data": {`,
		`    "name": "user"`,
		`  }`,
		`}`,
	)
	root := map[string]any{
		"home/user/.config/chezmoi/chezmoi.json": configFile,
	}
	chezmoitest.WithTestFS(t, root, func(fileSystem vfs.FS) {
		args := []string{"profile", "list"}
		stdout := strings.Builder{}
		config := newTestConfig(t, fileSystem, withStdout(&stdout))
		assert.NoError(t, config.execute(args))
		assert.True(t, strings.Contains(stdout.String(), "No profiles configured."), "got:\n%s", stdout.String())
	})
}

func TestProfileSource(t *testing.T) {
	for _, tc := range []struct {
		name         string
		envSet       bool
		envValue     string
		configFile   string
		cliArgs      []string
		expectSource profileSource
	}{
		{
			name:         "none",
			configFile:   `{"data": {}}`,
			expectSource: profileSourceNone,
		},
		{
			name:         "config_file",
			configFile:   `{"currentProfile": "dev", "profiles": {"dev": {}}}`,
			expectSource: profileSourceConfigFile,
		},
		{
			name:         "environment_overrides_config",
			envSet:       true,
			envValue:     "gpu",
			configFile:   `{"currentProfile": "dev", "profiles": {"dev": {}, "gpu": {}}}`,
			expectSource: profileSourceEnvironment,
		},
		{
			name:         "cli_overrides_all",
			envSet:       true,
			envValue:     "gpu",
			configFile:   `{"currentProfile": "dev", "profiles": {"dev": {}, "gpu": {}, "ci": {}}}`,
			cliArgs:      []string{"--profile=ci"},
			expectSource: profileSourceCommandLine,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CHEZMOI_PROFILE", "")
			root := map[string]any{
				"home/user/.config/chezmoi/chezmoi.json": tc.configFile,
			}
			chezmoitest.WithTestFS(t, root, func(fileSystem vfs.FS) {
				if tc.envSet {
					t.Setenv("CHEZMOI_PROFILE", tc.envValue)
				}
				args := append(tc.cliArgs, "profile", "--format=json")
				stdout := strings.Builder{}
				config := newTestConfig(t, fileSystem, withStdout(&stdout))
				assert.NoError(t, config.execute(args))

				var data struct {
					Source string `json:"source"`
				}
				assert.NoError(t, json.Unmarshal([]byte(stdout.String()), &data))
				assert.Equal(t, tc.expectSource.String(), data.Source)
			})
		})
	}
}
