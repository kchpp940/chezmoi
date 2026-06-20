package cmd

import (
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestNewSecretCacheKey(t *testing.T) {
	t.Run("same_parts_same_key", func(t *testing.T) {
		key1 := newSecretCacheKey("vault", "kv", "get", "-format=json", "secret/data/app")
		key2 := newSecretCacheKey("vault", "kv", "get", "-format=json", "secret/data/app")
		assert.Equal(t, key1, key2)
	})

	t.Run("different_command_different_key", func(t *testing.T) {
		key1 := newSecretCacheKey("vault", "kv", "get", "-format=json", "secret/data/app")
		key2 := newSecretCacheKey("vault2", "kv", "get", "-format=json", "secret/data/app")
		assert.NotEqual(t, key1, key2)
	})

	t.Run("different_args_different_key", func(t *testing.T) {
		key1 := newSecretCacheKey("doppler", "secrets", "download", "--json", "--no-file", "--project", "prod")
		key2 := newSecretCacheKey("doppler", "secrets", "download", "--json", "--no-file", "--project", "dev")
		assert.NotEqual(t, key1, key2)
	})

	t.Run("separator_in_value_no_collision", func(t *testing.T) {
		key1 := newSecretCacheKey("a\x00b", "c")
		key2 := newSecretCacheKey("a", "b\x00c")
		assert.NotEqual(t, key1, key2)
	})

	t.Run("different_part_count_no_collision", func(t *testing.T) {
		key1 := newSecretCacheKey("a", "bc")
		key2 := newSecretCacheKey("ab", "c")
		assert.NotEqual(t, key1, key2)
	})

	t.Run("empty_parts", func(t *testing.T) {
		key := newSecretCacheKey()
		assert.Equal(t, "", key)
	})
}

func TestSecretCacheKeyIsolation(t *testing.T) {
	t.Run("aws_region_isolation", func(t *testing.T) {
		key1 := awsSecretsManagerCacheKey{region: "us-east-1", profile: "default", arn: "arn:aws:secretsmanager:us-east-1:123456:secret:mysecret"}
		key2 := awsSecretsManagerCacheKey{region: "eu-west-1", profile: "default", arn: "arn:aws:secretsmanager:us-east-1:123456:secret:mysecret"}
		assert.NotEqual(t, key1, key2)
	})

	t.Run("aws_profile_isolation", func(t *testing.T) {
		key1 := awsSecretsManagerCacheKey{region: "us-east-1", profile: "prod", arn: "arn:aws:secretsmanager:us-east-1:123456:secret:mysecret"}
		key2 := awsSecretsManagerCacheKey{region: "us-east-1", profile: "dev", arn: "arn:aws:secretsmanager:us-east-1:123456:secret:mysecret"}
		assert.NotEqual(t, key1, key2)
	})

	t.Run("ejson_key_isolation", func(t *testing.T) {
		key1 := ejsonCacheKey{filePath: "/secrets/app.ejson", keyDir: "/keys", key: "key1"}
		key2 := ejsonCacheKey{filePath: "/secrets/app.ejson", keyDir: "/keys", key: "key2"}
		assert.NotEqual(t, key1, key2)
	})

	t.Run("ejson_keydir_isolation", func(t *testing.T) {
		key1 := ejsonCacheKey{filePath: "/secrets/app.ejson", keyDir: "/keys/dir1", key: "key1"}
		key2 := ejsonCacheKey{filePath: "/secrets/app.ejson", keyDir: "/keys/dir2", key: "key1"}
		assert.NotEqual(t, key1, key2)
	})

	t.Run("gopass_mode_isolation", func(t *testing.T) {
		key1 := gopassCacheKey{command: "gopass", mode: gopassModeBuiltin, id: "secret/app"}
		key2 := gopassCacheKey{command: "gopass", mode: gopassModeDefault, id: "secret/app"}
		assert.NotEqual(t, key1, key2)
	})

	t.Run("gopass_command_isolation", func(t *testing.T) {
		key1 := gopassCacheKey{command: "gopass", mode: gopassModeDefault, id: "secret/app"}
		key2 := gopassCacheKey{command: "gopass2", mode: gopassModeDefault, id: "secret/app"}
		assert.NotEqual(t, key1, key2)
	})

	t.Run("keepassxc_database_isolation", func(t *testing.T) {
		key1 := keepassxcCacheKey{database: "/db1.kdbx", mode: keepassxcModeCachePassword, entry: "entry1"}
		key2 := keepassxcCacheKey{database: "/db2.kdbx", mode: keepassxcModeCachePassword, entry: "entry1"}
		assert.NotEqual(t, key1, key2)
	})

	t.Run("keepassxc_mode_isolation", func(t *testing.T) {
		key1 := keepassxcCacheKey{database: "/db.kdbx", mode: keepassxcModeCachePassword, entry: "entry1"}
		key2 := keepassxcCacheKey{database: "/db.kdbx", mode: keepassxcModeOpen, entry: "entry1"}
		assert.NotEqual(t, key1, key2)
	})

	t.Run("keepassxc_attribute_database_isolation", func(t *testing.T) {
		key1 := keepassxcAttributeCacheKey{database: "/db1.kdbx", mode: keepassxcModeBuiltin, entry: "entry1", attribute: "Password"}
		key2 := keepassxcAttributeCacheKey{database: "/db2.kdbx", mode: keepassxcModeBuiltin, entry: "entry1", attribute: "Password"}
		assert.NotEqual(t, key1, key2)
	})
}

func TestClearSecretCaches(t *testing.T) {
	config := &Config{}

	config.AWSSecretsManager.cache = map[awsSecretsManagerCacheKey]string{
		{region: "us-east-1", profile: "default", arn: "test"}: "secret1",
	}
	config.AWSSecretsManager.jsonCache = map[awsSecretsManagerCacheKey]map[string]any{
		{region: "us-east-1", profile: "default", arn: "test"}: {"key": "val"},
	}
	config.Ejson.cache = map[ejsonCacheKey]any{
		{filePath: "/test.ejson", keyDir: "/keys", key: "k1"}: "val1",
	}
	config.Gopass.cache = map[gopassCacheKey]string{
		{command: "gopass", mode: gopassModeDefault, id: "test"}: "password1",
	}
	config.Keepassxc.cache = map[keepassxcCacheKey]map[string]string{
		{database: "/db.kdbx", mode: keepassxcModeBuiltin, entry: "test"}: {"Password": "pass1"},
	}
	config.Secret.cache = map[string][]byte{
		newSecretCacheKey("cmd", "arg1"): []byte("output1"),
	}

	config.clearSecretCaches()

	assert.Equal(t, nil, config.AWSSecretsManager.cache)
	assert.Equal(t, nil, config.AWSSecretsManager.jsonCache)
	assert.Equal(t, nil, config.Ejson.cache)
	assert.Equal(t, nil, config.Gopass.cache)
	assert.Equal(t, nil, config.Keepassxc.cache)
	assert.Equal(t, nil, config.Secret.cache)
}
