package cmd

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"chezmoi.io/chezmoi/v2/internal/chezmoi"
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

func TestSecretCacheKeyerInterface(t *testing.T) {
	t.Run("secret_config_command_isolation", func(t *testing.T) {
		c1 := &secretConfig{Command: "cmd1"}
		c2 := &secretConfig{Command: "cmd2"}
		assert.NotEqual(t, c1.secretCacheKey("arg1"), c2.secretCacheKey("arg1"))
	})

	t.Run("secret_config_args_isolation", func(t *testing.T) {
		c1 := &secretConfig{Command: "cmd", Args: []string{"--env", "prod"}}
		c2 := &secretConfig{Command: "cmd", Args: []string{"--env", "dev"}}
		assert.NotEqual(t, c1.secretCacheKey("arg1"), c2.secretCacheKey("arg1"))
	})

	t.Run("doppler_project_isolation", func(t *testing.T) {
		c1 := &dopplerConfig{Command: "doppler", Project: "project1", Config: "config1"}
		c2 := &dopplerConfig{Command: "doppler", Project: "project2", Config: "config1"}
		assert.NotEqual(t, c1.secretCacheKey(), c2.secretCacheKey())
	})

	t.Run("doppler_config_isolation", func(t *testing.T) {
		c1 := &dopplerConfig{Command: "doppler", Project: "project1", Config: "config1"}
		c2 := &dopplerConfig{Command: "doppler", Project: "project1", Config: "config2"}
		assert.NotEqual(t, c1.secretCacheKey(), c2.secretCacheKey())
	})

	t.Run("doppler_args_isolation", func(t *testing.T) {
		c1 := &dopplerConfig{Command: "doppler", Args: []string{"--token", "token1"}}
		c2 := &dopplerConfig{Command: "doppler", Args: []string{"--token", "token2"}}
		assert.NotEqual(t, c1.secretCacheKey(), c2.secretCacheKey())
	})

	t.Run("aws_region_isolation", func(t *testing.T) {
		c1 := &awsSecretsManagerConfig{Region: "us-east-1", Profile: "default"}
		c2 := &awsSecretsManagerConfig{Region: "eu-west-1", Profile: "default"}
		assert.NotEqual(t, c1.secretCacheKey("arn1"), c2.secretCacheKey("arn1"))
	})

	t.Run("aws_profile_isolation", func(t *testing.T) {
		c1 := &awsSecretsManagerConfig{Region: "us-east-1", Profile: "prod"}
		c2 := &awsSecretsManagerConfig{Region: "us-east-1", Profile: "dev"}
		assert.NotEqual(t, c1.secretCacheKey("arn1"), c2.secretCacheKey("arn1"))
	})

	t.Run("vault_command_isolation", func(t *testing.T) {
		c1 := &vaultConfig{Command: "vault"}
		c2 := &vaultConfig{Command: "vault2"}
		assert.NotEqual(t, c1.secretCacheKey("kv", "get", "secret/key"), c2.secretCacheKey("kv", "get", "secret/key"))
	})

	t.Run("pass_command_isolation", func(t *testing.T) {
		c1 := &passConfig{Command: "pass"}
		c2 := &passConfig{Command: "pass2"}
		assert.NotEqual(t, c1.secretCacheKey("show", "secret/key"), c2.secretCacheKey("show", "secret/key"))
	})

	t.Run("lastpass_command_isolation", func(t *testing.T) {
		c1 := &lastpassConfig{Command: "lpass"}
		c2 := &lastpassConfig{Command: "lpass2"}
		assert.NotEqual(t, c1.secretCacheKey("show", "id1"), c2.secretCacheKey("show", "id1"))
	})

	t.Run("bitwarden_command_isolation", func(t *testing.T) {
		c1 := &bitwardenConfig{Command: "bw"}
		c2 := &bitwardenConfig{Command: "bw2"}
		assert.NotEqual(t, c1.secretCacheKey("get", "id1"), c2.secretCacheKey("get", "id1"))
	})

	t.Run("bitwarden_unlock_isolation", func(t *testing.T) {
		var unlockAuto autoBool
		_ = unlockAuto.Set("auto")
		var unlockTrue autoBool
		_ = unlockTrue.Set("true")
		c1 := &bitwardenConfig{Command: "bw", Unlock: unlockAuto}
		c2 := &bitwardenConfig{Command: "bw", Unlock: unlockTrue}
		assert.NotEqual(t, c1.secretCacheKey("get", "id1"), c2.secretCacheKey("get", "id1"))
	})

	t.Run("bitwardensecrets_command_isolation", func(t *testing.T) {
		c1 := &bitwardenSecretsConfig{Command: "bws"}
		c2 := &bitwardenSecretsConfig{Command: "bws2"}
		assert.NotEqual(t, c1.secretCacheKey("secret", "get", "id1"), c2.secretCacheKey("secret", "get", "id1"))
	})

	t.Run("dashlane_command_isolation", func(t *testing.T) {
		c1 := &dashlaneConfig{Command: "dcli"}
		c2 := &dashlaneConfig{Command: "dcli2"}
		assert.NotEqual(t, c1.secretCacheKey("password", "filter"), c2.secretCacheKey("password", "filter"))
	})

	t.Run("dashlane_args_isolation", func(t *testing.T) {
		c1 := &dashlaneConfig{Command: "dcli", Args: []string{"--sync", "manual"}}
		c2 := &dashlaneConfig{Command: "dcli", Args: []string{"--sync", "auto"}}
		assert.NotEqual(t, c1.secretCacheKey("password", "filter"), c2.secretCacheKey("password", "filter"))
	})

	t.Run("keeper_command_isolation", func(t *testing.T) {
		c1 := &keeperConfig{Command: "keeper"}
		c2 := &keeperConfig{Command: "keeper2"}
		assert.NotEqual(t, c1.secretCacheKey("get", "record1"), c2.secretCacheKey("get", "record1"))
	})

	t.Run("keeper_args_isolation", func(t *testing.T) {
		c1 := &keeperConfig{Command: "keeper", Args: []string{"--server", "server1"}}
		c2 := &keeperConfig{Command: "keeper", Args: []string{"--server", "server2"}}
		assert.NotEqual(t, c1.secretCacheKey("get", "record1"), c2.secretCacheKey("get", "record1"))
	})

	t.Run("onepassword_command_isolation", func(t *testing.T) {
		c1 := &onepasswordConfig{Command: "op", Mode: onepasswordModeService}
		c2 := &onepasswordConfig{Command: "op2", Mode: onepasswordModeService}
		assert.NotEqual(t, c1.secretCacheKey("item", "get", "item1"), c2.secretCacheKey("item", "get", "item1"))
	})

	t.Run("onepassword_mode_isolation", func(t *testing.T) {
		c1 := &onepasswordConfig{Command: "op", Mode: onepasswordModeService}
		c2 := &onepasswordConfig{Command: "op", Mode: onepasswordModeConnect}
		assert.NotEqual(t, c1.secretCacheKey("item", "get", "item1"), c2.secretCacheKey("item", "get", "item1"))
	})

	t.Run("onepassword_prompt_isolation", func(t *testing.T) {
		c1 := &onepasswordConfig{Command: "op", Mode: onepasswordModeAccount, Prompt: true}
		c2 := &onepasswordConfig{Command: "op", Mode: onepasswordModeAccount, Prompt: false}
		assert.NotEqual(t, c1.secretCacheKey("item", "get", "item1"), c2.secretCacheKey("item", "get", "item1"))
	})

	t.Run("gopass_command_isolation", func(t *testing.T) {
		c1 := &gopassConfig{Command: "gopass", Mode: gopassModeDefault}
		c2 := &gopassConfig{Command: "gopass2", Mode: gopassModeDefault}
		assert.NotEqual(t, c1.secretCacheKey("show", "id1"), c2.secretCacheKey("show", "id1"))
	})

	t.Run("gopass_mode_isolation", func(t *testing.T) {
		c1 := &gopassConfig{Command: "gopass", Mode: gopassModeDefault}
		c2 := &gopassConfig{Command: "gopass", Mode: gopassModeBuiltin}
		assert.NotEqual(t, c1.secretCacheKey("show", "id1"), c2.secretCacheKey("show", "id1"))
	})

	t.Run("ejson_keydir_isolation", func(t *testing.T) {
		c1 := &ejsonConfig{KeyDir: "/keys1", Key: "key1"}
		c2 := &ejsonConfig{KeyDir: "/keys2", Key: "key1"}
		assert.NotEqual(t, c1.secretCacheKey("/file.ejson"), c2.secretCacheKey("/file.ejson"))
	})

	t.Run("ejson_key_isolation", func(t *testing.T) {
		c1 := &ejsonConfig{KeyDir: "/keys", Key: "key1"}
		c2 := &ejsonConfig{KeyDir: "/keys", Key: "key2"}
		assert.NotEqual(t, c1.secretCacheKey("/file.ejson"), c2.secretCacheKey("/file.ejson"))
	})

	t.Run("keepassxc_command_isolation", func(t *testing.T) {
		c1 := &keepassxcConfig{Command: "keepassxc-cli", Database: chezmoi.NewAbsPath("/db.kdbx"), Mode: keepassxcModeBuiltin}
		c2 := &keepassxcConfig{Command: "keepassxc-cli2", Database: chezmoi.NewAbsPath("/db.kdbx"), Mode: keepassxcModeBuiltin}
		assert.NotEqual(t, c1.secretCacheKey("entry1"), c2.secretCacheKey("entry1"))
	})

	t.Run("keepassxc_database_isolation", func(t *testing.T) {
		c1 := &keepassxcConfig{Command: "keepassxc-cli", Database: chezmoi.NewAbsPath("/db1.kdbx"), Mode: keepassxcModeBuiltin}
		c2 := &keepassxcConfig{Command: "keepassxc-cli", Database: chezmoi.NewAbsPath("/db2.kdbx"), Mode: keepassxcModeBuiltin}
		assert.NotEqual(t, c1.secretCacheKey("entry1"), c2.secretCacheKey("entry1"))
	})

	t.Run("keepassxc_mode_isolation", func(t *testing.T) {
		c1 := &keepassxcConfig{Command: "keepassxc-cli", Database: chezmoi.NewAbsPath("/db.kdbx"), Mode: keepassxcModeBuiltin}
		c2 := &keepassxcConfig{Command: "keepassxc-cli", Database: chezmoi.NewAbsPath("/db.kdbx"), Mode: keepassxcModeCachePassword}
		assert.NotEqual(t, c1.secretCacheKey("entry1"), c2.secretCacheKey("entry1"))
	})

	t.Run("keepassxc_args_isolation", func(t *testing.T) {
		c1 := &keepassxcConfig{Command: "keepassxc-cli", Database: chezmoi.NewAbsPath("/db.kdbx"), Mode: keepassxcModeCachePassword, Args: []string{"--no-password"}}
		c2 := &keepassxcConfig{Command: "keepassxc-cli", Database: chezmoi.NewAbsPath("/db.kdbx"), Mode: keepassxcModeCachePassword, Args: []string{"--key-file", "/key"}}
		assert.NotEqual(t, c1.secretCacheKey("entry1"), c2.secretCacheKey("entry1"))
	})

	t.Run("keepassxc_prompt_isolation", func(t *testing.T) {
		c1 := &keepassxcConfig{Command: "keepassxc-cli", Database: chezmoi.NewAbsPath("/db.kdbx"), Mode: keepassxcModeCachePassword, Prompt: true}
		c2 := &keepassxcConfig{Command: "keepassxc-cli", Database: chezmoi.NewAbsPath("/db.kdbx"), Mode: keepassxcModeCachePassword, Prompt: false}
		assert.NotEqual(t, c1.secretCacheKey("entry1"), c2.secretCacheKey("entry1"))
	})

	t.Run("passhole_command_isolation", func(t *testing.T) {
		c1 := &passholeConfig{Command: "ph", Prompt: false}
		c2 := &passholeConfig{Command: "ph2", Prompt: false}
		assert.NotEqual(t, c1.secretCacheKey("show", "--field", "password", "path"), c2.secretCacheKey("show", "--field", "password", "path"))
	})

	t.Run("passhole_args_isolation", func(t *testing.T) {
		c1 := &passholeConfig{Command: "ph", Args: []string{"--database", "/db1.kdbx"}, Prompt: false}
		c2 := &passholeConfig{Command: "ph", Args: []string{"--database", "/db2.kdbx"}, Prompt: false}
		assert.NotEqual(t, c1.secretCacheKey("show", "--field", "password", "path"), c2.secretCacheKey("show", "--field", "password", "path"))
	})

	t.Run("passhole_prompt_isolation", func(t *testing.T) {
		c1 := &passholeConfig{Command: "ph", Prompt: true}
		c2 := &passholeConfig{Command: "ph", Prompt: false}
		assert.NotEqual(t, c1.secretCacheKey("show", "--field", "password", "path"), c2.secretCacheKey("show", "--field", "password", "path"))
	})

	t.Run("protonpass_command_isolation", func(t *testing.T) {
		c1 := &protonPassConfig{Command: "protonpass"}
		c2 := &protonPassConfig{Command: "protonpass2"}
		assert.NotEqual(t, c1.secretCacheKey("item", "view", "item1"), c2.secretCacheKey("item", "view", "item1"))
	})

	t.Run("rbw_command_isolation", func(t *testing.T) {
		c1 := &rbwConfig{Command: "rbw"}
		c2 := &rbwConfig{Command: "rbw2"}
		assert.NotEqual(t, c1.secretCacheKey("get", "name1"), c2.secretCacheKey("get", "name1"))
	})
}
