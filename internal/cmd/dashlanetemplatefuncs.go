package cmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"slices"

	"chezmoi.io/chezmoi/v2/internal/chezmoilog"
)

type dashlaneConfig struct {
	Command     string   `json:"command" mapstructure:"command" yaml:"command"`
	Args        []string `json:"args"    mapstructure:"args"    yaml:"args"`
	outputCache map[string][]byte
}

func (c *dashlaneConfig) secretCacheKey(extraParts ...string) string {
	parts := make([]string, 0, 1+len(c.Args)+len(extraParts))
	parts = append(parts, c.Command)
	parts = append(parts, c.Args...)
	parts = append(parts, extraParts...)
	return newSecretCacheKey(parts...)
}

func (c *Config) dashlaneNoteTemplateFunc(filter string) any {
	output := mustValue(c.dashlaneOutput("note", filter))
	return string(output)
}

func (c *Config) dashlanePasswordTemplateFunc(filter string) any {
	output := mustValue(c.dashlaneOutput("password", "--output", "json", filter))

	var data any
	must(json.Unmarshal(output, &data))

	return data
}

func (c *Config) dashlaneOutput(args ...string) ([]byte, error) {
	key := c.Dashlane.secretCacheKey(args...)
	if output, ok := c.Dashlane.outputCache[key]; ok {
		return output, nil
	}

	name := c.Dashlane.Command
	fullArgs := append(slices.Clone(c.Dashlane.Args), args...)
	cmd := exec.Command(name, fullArgs...)
	cmd.Stderr = os.Stderr
	output, err := chezmoilog.LogCmdOutput(c.logger, cmd)
	if err != nil {
		return nil, err
	}

	if c.Dashlane.outputCache == nil {
		c.Dashlane.outputCache = make(map[string][]byte)
	}
	c.Dashlane.outputCache[key] = output

	return output, nil
}
