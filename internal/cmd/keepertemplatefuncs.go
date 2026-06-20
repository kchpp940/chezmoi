package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"slices"
	"strings"

	"chezmoi.io/chezmoi/v2/internal/chezmoilog"
)

type keeperConfig struct {
	Command     string   `json:"command" mapstructure:"command" yaml:"command"`
	Args        []string `json:"args"    mapstructure:"args"    yaml:"args"`
	outputCache map[string][]byte
}

func (c *Config) keeperTemplateFunc(record string) map[string]any {
	output := mustValue(c.keeperOutput([]string{"get", "--format=json", record}))
	var result map[string]any
	must(json.Unmarshal(output, &result))
	return result
}

func (c *Config) keeperDataFieldsTemplateFunc(record string) map[string]any {
	output := mustValue(c.keeperOutput([]string{"get", "--format=json", record}))
	var data struct {
		Data struct {
			Fields []struct {
				Type  string `json:"type"`
				Value any    `json:"value"`
			} `json:"fields"`
		} `json:"data"`
	}
	must(json.Unmarshal(output, &data))
	result := make(map[string]any)
	for _, field := range data.Data.Fields {
		result[field.Type] = field.Value
	}
	return result
}

func (c *Config) keeperFindPasswordTemplateFunc(record string) string {
	output := mustValue(c.keeperOutput([]string{"find-password", record}))
	return string(bytes.TrimSpace(output))
}

func (c *Config) keeperOutput(args []string) ([]byte, error) {
	fullArgs := append(slices.Clone(args), c.Keeper.Args...)
	key := strings.Join(fullArgs, "\x00")
	if data, ok := c.Keeper.outputCache[key]; ok {
		return data, nil
	}

	name := c.Keeper.Command
	cmd := exec.Command(name, fullArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	output, err := chezmoilog.LogCmdOutput(c.logger, cmd)
	if err != nil {
		return nil, newCmdOutputError(cmd, output, err)
	}

	if c.Keeper.outputCache == nil {
		c.Keeper.outputCache = make(map[string][]byte)
	}
	c.Keeper.outputCache[key] = output
	return output, nil
}
