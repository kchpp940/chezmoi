package cmd

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"os/exec"
	"slices"

	"chezmoi.io/chezmoi/v2/internal/chezmoilog"
)

// secretCacheKeyer is the interface implemented by secret provider configs
// that need cache isolation based on their configuration context.
// Implementations MUST explicitly list ALL configuration fields that affect
// secret retrieval (e.g., command, region, profile, vault, account, args, etc.).
// When adding new configuration fields to a provider, update this method
// to include them if they affect the secret retrieval result.
type secretCacheKeyer interface {
	secretCacheKey(extraParts ...string) string
}

// secretCacheResetter is the interface implemented by secret provider configs
// that need to reset their cache and client state on config reload.
// Implementations MUST reset ALL runtime state including:
//   - Output caches (cache, outputCache, jsonCache, etc.)
//   - Client connections (svcs, client, cred, console, cmd, etc.)
//   - Session tokens and credentials (session, password, sessionTokens, etc.)
//   - Lazy-initialized state flags (modeChecked, accountMap, etc.)
//
// When adding new state fields to a provider, add them to this method.
type secretCacheResetter interface {
	resetSecretCache()
}

func newSecretCacheKey(parts ...string) string {
	var buf bytes.Buffer
	for _, part := range parts {
		_ = binary.Write(&buf, binary.BigEndian, uint32(len(part)))
		buf.WriteString(part)
	}
	return buf.String()
}

type secretConfig struct {
	Command string   `json:"command" mapstructure:"command" yaml:"command"`
	Args    []string `json:"args"    mapstructure:"args"    yaml:"args"`
	cache   map[string][]byte
}

func (c *secretConfig) secretCacheKey(extraParts ...string) string {
	parts := make([]string, 0, 1+len(c.Args)+len(extraParts))
	parts = append(parts, c.Command)
	parts = append(parts, c.Args...)
	parts = append(parts, extraParts...)
	return newSecretCacheKey(parts...)
}

func (c *secretConfig) resetSecretCache() {
	c.cache = nil
}

func (c *Config) secretTemplateFunc(args ...string) string {
	return string(bytes.TrimSpace(mustValue(c.secretOutput(args))))
}

func (c *Config) secretJSONTemplateFunc(args ...string) any {
	output := mustValue(c.secretOutput(args))
	var value any
	must(json.Unmarshal(output, &value))
	return value
}

func (c *Config) secretOutput(args []string) ([]byte, error) {
	key := c.Secret.secretCacheKey(args...)
	if output, ok := c.Secret.cache[key]; ok {
		return output, nil
	}

	fullArgs := append(slices.Clone(c.Secret.Args), args...)
	cmd := exec.Command(c.Secret.Command, fullArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	output, err := chezmoilog.LogCmdOutput(c.logger, cmd)
	if err != nil {
		return nil, newCmdOutputError(cmd, output, err)
	}

	if c.Secret.cache == nil {
		c.Secret.cache = make(map[string][]byte)
	}
	c.Secret.cache[key] = output

	return output, nil
}

func (c *Config) clearSecretCaches() {
	// All secret providers implement secretCacheResetter.
	// Their cleanup logic is localized in each provider's resetSecretCache() method.
	// When adding a new provider, implement resetSecretCache() and add it here.
	resetters := []secretCacheResetter{
		&c.AWSSecretsManager,
		&c.AzureKeyVault,
		&c.Bitwarden,
		&c.BitwardenSecrets,
		&c.Dashlane,
		&c.Doppler,
		&c.Ejson,
		&c.Gopass,
		&c.Keepassxc,
		&c.Keeper,
		&c.Lastpass,
		&c.Onepassword,
		&c.Pass,
		&c.Passhole,
		&c.ProtonPass,
		&c.RBW,
		&c.Secret,
		&c.Vault,
		&c.keyring,
	}
	for _, resetter := range resetters {
		resetter.resetSecretCache()
	}

	// GitHub is not a secret provider but has similar lifecycle needs.
	c.gitHub.client = nil
	c.gitHub.clientErr = nil
	c.gitHub.keysCache = nil
	c.gitHub.versionReleaseCache = nil
	c.gitHub.latestReleaseCache = nil
	c.gitHub.releasesCache = nil
	c.gitHub.tagsCache = nil
}
