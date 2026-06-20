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
	fullArgs := append(slices.Clone(c.Dashlane.Args), args...)
	key := newSecretCacheKey(fullArgs...)
	if output, ok := c.Dashlane.outputCache[key]; ok {
		return output, nil
	}

	name := c.Dashlane.Command
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
