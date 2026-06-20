package cmd

import (
	"encoding/json"

	"github.com/Shopify/ejson"
)

type ejsonCacheKey struct {
	filePath string
	keyDir   string
	key      string
}

type ejsonConfig struct {
	KeyDir string `json:"keyDir" mapstructure:"keyDir" yaml:"keyDir"`
	Key    string `json:"key"    mapstructure:"key"    yaml:"key"`
	cache  map[ejsonCacheKey]any
}

func (c *Config) ejsonDecryptWithKeyTemplateFunc(filePath, key string) any {
	cacheKey := ejsonCacheKey{
		filePath: filePath,
		keyDir:   c.Ejson.KeyDir,
		key:      key,
	}
	if data, ok := c.Ejson.cache[cacheKey]; ok {
		return data
	}

	if c.Ejson.cache == nil {
		c.Ejson.cache = make(map[ejsonCacheKey]any)
	}

	decrypted := mustValue(ejson.DecryptFile(filePath, c.Ejson.KeyDir, key))

	var data any
	must(json.Unmarshal(decrypted, &data))

	c.Ejson.cache[cacheKey] = data

	return data
}

func (c *Config) ejsonDecryptTemplateFunc(filePath string) any {
	return c.ejsonDecryptWithKeyTemplateFunc(filePath, c.Ejson.Key)
}
