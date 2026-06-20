package cmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"

	"chezmoi.io/chezmoi/v2/internal/chezmoilog"
)

type vaultConfig struct {
	Command string `json:"command" mapstructure:"command" yaml:"command"`
	cache   map[string]any
}

func (c *Config) vaultTemplateFunc(key string) any {
	cacheKey := strings.Join([]string{c.Vault.Command, "kv", "get", "-format=json", key}, "\x00")
	if data, ok := c.Vault.cache[cacheKey]; ok {
		return data
	}

	args := []string{"kv", "get", "-format=json", key}
	cmd := exec.Command(c.Vault.Command, args...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	output, err := chezmoilog.LogCmdOutput(c.logger, cmd)
	if err != nil {
		panic(newCmdOutputError(cmd, output, err))
	}

	var data any
	must(json.Unmarshal(output, &data))

	if c.Vault.cache == nil {
		c.Vault.cache = make(map[string]any)
	}
	c.Vault.cache[cacheKey] = data

	return data
}
