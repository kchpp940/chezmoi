//go:build !freebsd || (freebsd && cgo)

package cmd

import (
	"fmt"

	"github.com/zalando/go-keyring"
)

type keyringKey struct {
	service string
	user    string
}

type keyringData struct {
	cache map[string]string
}

func (c *keyringData) secretCacheKey(extraParts ...string) string {
	return newSecretCacheKey(extraParts...)
}

func (c *keyringData) resetSecretCache() {
	c.cache = nil
}

func (c *Config) keyringTemplateFunc(service, user string) string {
	cacheKey := c.keyring.secretCacheKey(service, user)
	if password, ok := c.keyring.cache[cacheKey]; ok {
		return password
	}
	password, err := keyring.Get(service, user)
	if err != nil {
		panic(fmt.Errorf("%s %s: %w", service, user, err))
	}

	if c.keyring.cache == nil {
		c.keyring.cache = make(map[string]string)
	}

	c.keyring.cache[cacheKey] = password
	return password
}
