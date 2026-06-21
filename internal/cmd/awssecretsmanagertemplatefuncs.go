package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type awsSecretsManagerCacheKey struct {
	region  string
	profile string
	arn     string
}

type awsSecretsManagerConfig struct {
	Region    string `json:"region"  mapstructure:"region"  yaml:"region"`
	Profile   string `json:"profile" mapstructure:"profile" yaml:"profile"`
	svcs      map[string]*secretsmanager.Client
	cache     map[awsSecretsManagerCacheKey]string
	jsonCache map[awsSecretsManagerCacheKey]map[string]any
}

func (c *awsSecretsManagerConfig) secretCacheKey(extraParts ...string) string {
	parts := make([]string, 0, 2+len(extraParts))
	parts = append(parts, c.Region, c.Profile)
	parts = append(parts, extraParts...)
	return newSecretCacheKey(parts...)
}

func (c *awsSecretsManagerConfig) resetSecretCache() {
	c.svcs = nil
	c.cache = nil
	c.jsonCache = nil
}

func awsSecretsManagerSvcCacheKey(region, profile string) string {
	return newSecretCacheKey(region, profile)
}

func (c *Config) awsSecretsManagerRawTemplateFunc(arn string) string {
	key := awsSecretsManagerCacheKey{
		region:  c.AWSSecretsManager.Region,
		profile: c.AWSSecretsManager.Profile,
		arn:     arn,
	}
	if secret, ok := c.AWSSecretsManager.cache[key]; ok {
		return secret
	}

	svcKey := awsSecretsManagerSvcCacheKey(c.AWSSecretsManager.Region, c.AWSSecretsManager.Profile)
	if c.AWSSecretsManager.svcs == nil {
		c.AWSSecretsManager.svcs = make(map[string]*secretsmanager.Client)
	}
	svc, ok := c.AWSSecretsManager.svcs[svcKey]
	if !ok {
		var opts []func(*config.LoadOptions) error
		if region := c.AWSSecretsManager.Region; region != "" {
			opts = append(opts, config.WithRegion(region))
		}
		if profile := c.AWSSecretsManager.Profile; profile != "" {
			opts = append(opts, config.WithSharedConfigProfile(profile))
		}

		opts = append(opts, config.WithRetryMaxAttempts(1))

		cfg, err := config.LoadDefaultConfig(context.Background(), opts...)
		if err != nil {
			panic(err)
		}

		svc = secretsmanager.NewFromConfig(cfg)
		c.AWSSecretsManager.svcs[svcKey] = svc
	}

	result, err := svc.GetSecretValue(context.Background(), &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(arn),
	})
	if err != nil {
		panic(fmt.Errorf("aws secrets manager %s: %w", arn, err))
	}

	var secret string
	if result.SecretString != nil {
		secret = *result.SecretString
	} else {
		decodedBinarySecretBytes := make([]byte, base64.StdEncoding.DecodedLen(len(result.SecretBinary)))
		length, err := base64.StdEncoding.Decode(decodedBinarySecretBytes, result.SecretBinary)
		if err != nil {
			panic(err)
		}

		secret = string(decodedBinarySecretBytes[:length])
	}

	if c.AWSSecretsManager.cache == nil {
		c.AWSSecretsManager.cache = make(map[awsSecretsManagerCacheKey]string)
	}

	c.AWSSecretsManager.cache[key] = secret
	return secret
}

func (c *Config) awsSecretsManagerTemplateFunc(arn string) map[string]any {
	key := awsSecretsManagerCacheKey{
		region:  c.AWSSecretsManager.Region,
		profile: c.AWSSecretsManager.Profile,
		arn:     arn,
	}
	if secret, ok := c.AWSSecretsManager.jsonCache[key]; ok {
		return secret
	}

	raw := c.awsSecretsManagerRawTemplateFunc(arn)

	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		panic(err)
	}

	if c.AWSSecretsManager.jsonCache == nil {
		c.AWSSecretsManager.jsonCache = make(map[awsSecretsManagerCacheKey]map[string]any)
	}

	c.AWSSecretsManager.jsonCache[key] = data
	return data
}
