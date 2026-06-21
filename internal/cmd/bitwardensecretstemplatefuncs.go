package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"chezmoi.io/chezmoi/v2/internal/chezmoilog"
)

type bitwardenSecretsConfig struct {
	Command     string `json:"command" mapstructure:"command" yaml:"command"`
	outputCache map[string][]byte
}

func (c *bitwardenSecretsConfig) secretCacheKey(extraParts ...string) string {
	parts := make([]string, 0, 1+len(extraParts))
	parts = append(parts, c.Command)
	parts = append(parts, extraParts...)
	return newSecretCacheKey(parts...)
}

func (c *bitwardenSecretsConfig) resetSecretCache() {
	c.outputCache = nil
}

func (c *Config) bitwardenSecretsTemplateFunc(secretID string, additionalArgs ...string) any {
	args := []string{"secret", "get", secretID}
	switch len(additionalArgs) {
	case 0:
		// Do nothing.
	case 1:
		args = append(args, "--access-token", additionalArgs[0])
	default:
		panic(fmt.Errorf("expected 1 or 2 arguments, got %d", len(additionalArgs)+1))
	}
	output := mustValue(c.bitwardenSecretsOutput(args))
	var data map[string]any
	must(json.Unmarshal(output, &data))
	return data
}

func (c *Config) bitwardenSecretsOutput(args []string) ([]byte, error) {
	key := c.BitwardenSecrets.secretCacheKey(args...)
	if data, ok := c.BitwardenSecrets.outputCache[key]; ok {
		return data, nil
	}

	name := c.BitwardenSecrets.Command
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	output, err := chezmoilog.LogCmdOutput(c.logger, cmd)
	if err != nil {
		return nil, newCmdOutputError(cmd, output, err)
	}

	if c.BitwardenSecrets.outputCache == nil {
		c.BitwardenSecrets.outputCache = make(map[string][]byte)
	}
	c.BitwardenSecrets.outputCache[key] = output
	return output, nil
}
