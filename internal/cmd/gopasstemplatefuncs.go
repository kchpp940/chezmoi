package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"

	"github.com/coreos/go-semver/semver"
	"github.com/gopasspw/gopass/pkg/ctxutil"
	"github.com/gopasspw/gopass/pkg/gopass"
	"github.com/gopasspw/gopass/pkg/gopass/api"

	"chezmoi.io/chezmoi/v2/internal/chezmoilog"
)

type gopassMode string

const (
	gopassModeBuiltin gopassMode = "builtin"
	gopassModeDefault gopassMode = ""
)

var (
	gopassMinVersion  = semver.Version{Major: 1, Minor: 6, Patch: 1}
	gopassVersionArgs = []string{"--version"}
	gopassVersionRx   = regexp.MustCompile(`gopass\s+(\d+\.\d+\.\d+)`)
)

type gopassCacheKey struct {
	command string
	mode    gopassMode
	id      string
}

type gopassConfig struct {
	Command       string          `json:"command" mapstructure:"command" yaml:"command"`
	Mode          gopassMode      `json:"mode"    mapstructure:"mode"    yaml:"mode"`
	ctx           context.Context //nolint:containedctx
	client        *api.Gopass
	clientErr     error
	passwordCache map[string][]byte
	cache         map[gopassCacheKey]string
	rawCache      map[gopassCacheKey][]byte
}

func (c *gopassConfig) secretCacheKey(extraParts ...string) string {
	parts := make([]string, 0, 2+len(extraParts))
	parts = append(parts, c.Command, string(c.Mode))
	parts = append(parts, extraParts...)
	return newSecretCacheKey(parts...)
}

func (c *Config) gopassTemplateFunc(id string) string {
	cacheKey := gopassCacheKey{
		command: c.Gopass.Command,
		mode:    c.Gopass.Mode,
		id:      id,
	}
	if password, ok := c.Gopass.cache[cacheKey]; ok {
		return password
	}

	var password string
	switch c.Gopass.Mode {
	case gopassModeBuiltin:
		password = mustValue(c.builtinGopassSecret(id, "latest")).Password()
	case gopassModeDefault:
		output := mustValue(c.gopassOutput("show", "--password", id))
		passwordBytes, _, _ := bytes.Cut(output, []byte{'\n'})
		password = string(passwordBytes)
	default:
		panic(fmt.Errorf("%s: invalid mode", c.Gopass.Mode))
	}

	if c.Gopass.cache == nil {
		c.Gopass.cache = make(map[gopassCacheKey]string)
	}
	c.Gopass.cache[cacheKey] = password

	return password
}

func (c *Config) gopassRawTemplateFunc(id string) string {
	cacheKey := gopassCacheKey{
		command: c.Gopass.Command,
		mode:    c.Gopass.Mode,
		id:      id,
	}
	if output, ok := c.Gopass.rawCache[cacheKey]; ok {
		return string(output)
	}

	var output []byte
	switch c.Gopass.Mode {
	case gopassModeBuiltin:
		output = mustValue(c.builtinGopassSecret(id, "latest")).Bytes()
	case gopassModeDefault:
		output = mustValue(c.gopassOutput("show", "--noparsing", id))
	default:
		panic(fmt.Errorf("%s: invalid mode", c.Gopass.Mode))
	}

	if c.Gopass.rawCache == nil {
		c.Gopass.rawCache = make(map[gopassCacheKey][]byte)
	}
	c.Gopass.rawCache[cacheKey] = output

	return string(output)
}

func (c *Config) builtinGopassClient() (*api.Gopass, error) {
	if c.Gopass.ctx != nil {
		return c.Gopass.client, c.Gopass.clientErr
	}
	ctx := context.Background()
	ctx = ctxutil.WithPasswordCallback(ctx, func(filename string, confirm bool) ([]byte, error) {
		if _, ok := c.Gopass.passwordCache[filename]; !ok {
			password, err := c.readPassword("Passphrase for "+filename+": ", "passphrase")
			if err != nil {
				return nil, err
			}
			if c.Gopass.passwordCache == nil {
				c.Gopass.passwordCache = make(map[string][]byte, 1)
			}
			c.Gopass.passwordCache[filename] = []byte(password)
		}
		return c.Gopass.passwordCache[filename], nil
	})
	c.Gopass.ctx = ctx //nolint:fatcontext
	c.Gopass.client, c.Gopass.clientErr = api.New(c.Gopass.ctx)
	return c.Gopass.client, c.Gopass.clientErr
}

func (c *Config) builtinGopassSecret(name, revision string) (gopass.Secret, error) {
	client, err := c.builtinGopassClient()
	if err != nil {
		return nil, err
	}
	secret, err := client.Get(c.Gopass.ctx, name, revision)
	if err != nil {
		return nil, err
	}
	return secret, nil
}

func (c *Config) gopassOutput(args ...string) ([]byte, error) {
	name := c.Gopass.Command
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	output, err := chezmoilog.LogCmdOutput(c.logger, cmd)
	if err != nil {
		return nil, newCmdOutputError(cmd, output, err)
	}
	return output, nil
}
