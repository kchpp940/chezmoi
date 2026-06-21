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
	// keyring 没有额外配置上下文，使用系统 keyring 的全局配置
	// 显式声明为固定上下文，避免游离在统一规范之外
	return newSecretCacheKey(extraParts...)
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
