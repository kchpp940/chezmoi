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
	fullArgs := append(slices.Clone(c.Secret.Args), args...)
	key := newSecretCacheKey(append([]string{c.Secret.Command}, fullArgs...)...)
	if output, ok := c.Secret.cache[key]; ok {
		return output, nil
	}

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
	c.AWSSecretsManager.svcs = nil
	c.AWSSecretsManager.cache = nil
	c.AWSSecretsManager.jsonCache = nil

	c.AzureKeyVault.vaults = nil
	c.AzureKeyVault.cred = nil

	c.Bitwarden.session = ""
	c.Bitwarden.outputCache = nil

	c.BitwardenSecrets.outputCache = nil

	c.Dashlane.outputCache = nil

	c.Doppler.outputCache = nil

	c.Ejson.cache = nil

	c.Gopass.ctx = nil
	c.Gopass.client = nil
	c.Gopass.clientErr = nil
	c.Gopass.passwordCache = nil
	c.Gopass.cache = nil
	c.Gopass.rawCache = nil

	c.Keepassxc.cmd = nil
	c.Keepassxc.console = nil
	c.Keepassxc.promptStr = ""
	c.Keepassxc.cache = nil
	c.Keepassxc.attachmentCache = nil
	c.Keepassxc.attributeCache = nil
	c.Keepassxc.password = ""

	c.Keeper.outputCache = nil

	c.Lastpass.cache = nil

	c.Onepassword.outputCache = nil
	c.Onepassword.sessionTokens = nil
	c.Onepassword.accountMap = nil
	c.Onepassword.accountMapErr = nil
	c.Onepassword.modeChecked = false

	c.Pass.cache = nil

	c.Passhole.cache = nil
	c.Passhole.password = ""

	c.ProtonPass.outputCache = nil

	c.RBW.outputCache = nil

	c.Secret.cache = nil

	c.Vault.cache = nil

	c.gitHub.client = nil
	c.gitHub.clientErr = nil
	c.gitHub.keysCache = nil
	c.gitHub.versionReleaseCache = nil
	c.gitHub.latestReleaseCache = nil
	c.gitHub.releasesCache = nil
	c.gitHub.tagsCache = nil

	c.keyring.cache = nil
}
