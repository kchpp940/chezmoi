package cmd

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"slices"

	"github.com/coreos/go-semver/semver"

	"chezmoi.io/chezmoi/v2/internal/chezmoilog"
)

type passholeConfig struct {
	Command  string   `json:"command" mapstructure:"command" yaml:"command"`
	Args     []string `json:"args"    mapstructure:"args"    yaml:"args"`
	Prompt   bool     `json:"prompt"  mapstructure:"prompt"  yaml:"prompt"`
	cache    map[string]string
	password string
}

func (c *passholeConfig) secretCacheKey(extraParts ...string) string {
	parts := make([]string, 0, 2+len(c.Args)+len(extraParts))
	parts = append(parts, c.Command)
	parts = append(parts, c.Args...)
	if c.Prompt {
		parts = append(parts, "prompt=true", "--password", "-")
	} else {
		parts = append(parts, "prompt=false")
	}
	parts = append(parts, extraParts...)
	return newSecretCacheKey(parts...)
}

func (c *passholeConfig) resetSecretCache() {
	c.cache = nil
	c.password = ""
}

var passholeMinVersion = semver.Version{Major: 1, Minor: 10, Patch: 0}

func (c *Config) passholeTemplateFunc(path, field string) string {
	args := slices.Clone(c.Passhole.Args)
	var stdin io.Reader
	if c.Passhole.Prompt {
		if c.Passhole.password == "" {
			password := mustValue(c.readPassword("Enter database password: ", "password"))
			c.Passhole.password = password
		}
		args = append(args, "--password", "-")
		stdin = bytes.NewBufferString(c.Passhole.password + "\n")
	}
	args = append(args, "show", "--field", field, path)

	cacheKey := c.Passhole.secretCacheKey("show", "--field", field, path)
	if value, ok := c.Passhole.cache[cacheKey]; ok {
		return value
	}

	output := mustValue(c.passholeOutput(c.Passhole.Command, args, stdin))

	if c.Passhole.cache == nil {
		c.Passhole.cache = make(map[string]string)
	}
	c.Passhole.cache[cacheKey] = output
	return output
}

func (c *Config) passholeOutput(name string, args []string, stdin io.Reader) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Stdin = stdin
	cmd.Stderr = os.Stderr
	output, err := chezmoilog.LogCmdOutput(c.logger, cmd)
	if err != nil {
		return "", newCmdOutputError(cmd, output, err)
	}
	return string(output), nil
}
