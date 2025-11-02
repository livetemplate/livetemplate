package stack

import "fmt"

// Package stack provides types and configuration for deployment stack generation.
// It supports multiple cloud providers (Docker, Fly.io, DigitalOcean, Kubernetes)
// with configurable database, backup, caching, and storage options.

type Provider string

const (
	ProviderDocker       Provider = "docker"
	ProviderFly          Provider = "fly"
	ProviderDigitalOcean Provider = "do"
	ProviderK8s          Provider = "k8s"
)

type DatabaseType string

const (
	DatabaseSQLite   DatabaseType = "sqlite"
	DatabasePostgres DatabaseType = "postgres"
	DatabaseNone     DatabaseType = "none"
)

type BackupType string

const (
	BackupLitestream BackupType = "litestream"
	BackupNone       BackupType = "none"
)

type RedisType string

const (
	RedisUpstash RedisType = "upstash"
	RedisFly     RedisType = "fly"
	RedisNone    RedisType = "none"
)

type StorageType string

const (
	StorageS3   StorageType = "s3"
	StorageDO   StorageType = "do-spaces"
	StorageB2   StorageType = "b2"
	StorageNone StorageType = "none"
)

type CIType string

const (
	CIGitHub CIType = "github"
	CIGitLab CIType = "gitlab"
	CINone   CIType = "none"
)

type IngressType string

const (
	IngressNginx   IngressType = "nginx"
	IngressTraefik IngressType = "traefik"
	IngressNone    IngressType = "none"
)

type RegistryType string

const (
	RegistryGHCR   RegistryType = "ghcr"
	RegistryDocker RegistryType = "docker"
	RegistryGCR    RegistryType = "gcr"
	RegistryECR    RegistryType = "ecr"
)

type StackConfig struct {
	Provider    Provider
	Database    DatabaseType
	Backup      BackupType
	Redis       RedisType
	Storage     StorageType
	CI          CIType
	Namespace   string
	MultiRegion bool
	Ingress     IngressType
	Registry    RegistryType
}

func (c *StackConfig) Validate() error {
	validProviders := map[Provider]bool{
		ProviderDocker: true, ProviderFly: true,
		ProviderDigitalOcean: true, ProviderK8s: true,
	}
	if !validProviders[c.Provider] {
		return fmt.Errorf("invalid provider: %s. Valid: docker, fly, do, k8s", c.Provider)
	}

	if c.Backup == BackupLitestream && c.Storage == StorageNone {
		return fmt.Errorf("when --backup=litestream, --storage flag is required")
	}

	if c.Namespace != "" && c.Provider != ProviderK8s {
		return fmt.Errorf("--namespace only applies to k8s provider")
	}
	if c.Ingress != IngressNone && c.Ingress != "" && c.Provider != ProviderK8s {
		return fmt.Errorf("--ingress only applies to k8s provider")
	}
	if c.Registry != "" && c.Provider != ProviderK8s {
		return fmt.Errorf("--registry only applies to k8s provider")
	}

	return nil
}

type TemplateData struct {
	ProjectName string
	Provider    string
	Database    string
	Backup      string
	Redis       string
	Storage     string
	CI          string
	Namespace   string
	MultiRegion bool
	Ingress     string
	Registry    string
	Secrets     map[string]string
}

func (c *StackConfig) ToTemplateData(projectName string) *TemplateData {
	return &TemplateData{
		ProjectName: projectName,
		Provider:    string(c.Provider),
		Database:    string(c.Database),
		Backup:      string(c.Backup),
		Redis:       string(c.Redis),
		Storage:     string(c.Storage),
		CI:          string(c.CI),
		Namespace:   c.Namespace,
		MultiRegion: c.MultiRegion,
		Ingress:     string(c.Ingress),
		Registry:    string(c.Registry),
		Secrets:     make(map[string]string),
	}
}
