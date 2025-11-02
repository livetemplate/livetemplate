package stack

import "testing"

func TestStackConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  StackConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid sqlite with litestream",
			config: StackConfig{
				Provider: ProviderDocker,
				Database: DatabaseSQLite,
				Backup:   BackupLitestream,
				Storage:  StorageS3,
			},
			wantErr: false,
		},
		{
			name: "litestream without storage",
			config: StackConfig{
				Provider: ProviderDocker,
				Database: DatabaseSQLite,
				Backup:   BackupLitestream,
				Storage:  StorageNone,
			},
			wantErr: true,
			errMsg:  "when --backup=litestream, --storage flag is required",
		},
		{
			name: "postgres with backup ignored",
			config: StackConfig{
				Provider: ProviderDocker,
				Database: DatabasePostgres,
				Backup:   BackupLitestream,
			},
			wantErr: false,
		},
		{
			name: "invalid provider",
			config: StackConfig{
				Provider: Provider("invalid"),
			},
			wantErr: true,
		},
		{
			name: "namespace with non-k8s provider fails",
			config: StackConfig{
				Provider:  ProviderDocker,
				Namespace: "my-namespace",
			},
			wantErr: true,
			errMsg:  "--namespace only applies to k8s provider",
		},
		{
			name: "ingress with non-k8s provider fails",
			config: StackConfig{
				Provider: ProviderFly,
				Ingress:  IngressNginx,
			},
			wantErr: true,
			errMsg:  "--ingress only applies to k8s provider",
		},
		{
			name: "registry with non-k8s provider fails",
			config: StackConfig{
				Provider: ProviderDocker,
				Registry: RegistryGHCR,
			},
			wantErr: true,
			errMsg:  "--registry only applies to k8s provider",
		},
		{
			name: "k8s with namespace, ingress, and registry succeeds",
			config: StackConfig{
				Provider:  ProviderK8s,
				Namespace: "production",
				Ingress:   IngressTraefik,
				Registry:  RegistryGCR,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Errorf("Validate() error = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestStackConfig_ToTemplateData(t *testing.T) {
	config := StackConfig{
		Provider:    ProviderK8s,
		Database:    DatabasePostgres,
		Backup:      BackupLitestream,
		Redis:       RedisUpstash,
		Storage:     StorageS3,
		Namespace:   "production",
		MultiRegion: true,
		Ingress:     IngressNginx,
		Registry:    RegistryGHCR,
	}

	projectName := "my-project"
	data := config.ToTemplateData(projectName)

	if data.ProjectName != projectName {
		t.Errorf("ProjectName = %v, want %v", data.ProjectName, projectName)
	}
	if data.Provider != string(ProviderK8s) {
		t.Errorf("Provider = %v, want %v", data.Provider, string(ProviderK8s))
	}
	if data.Database != string(DatabasePostgres) {
		t.Errorf("Database = %v, want %v", data.Database, string(DatabasePostgres))
	}
	if data.Backup != string(BackupLitestream) {
		t.Errorf("Backup = %v, want %v", data.Backup, string(BackupLitestream))
	}
	if data.Redis != string(RedisUpstash) {
		t.Errorf("Redis = %v, want %v", data.Redis, string(RedisUpstash))
	}
	if data.Storage != string(StorageS3) {
		t.Errorf("Storage = %v, want %v", data.Storage, string(StorageS3))
	}
	if data.Namespace != "production" {
		t.Errorf("Namespace = %v, want %v", data.Namespace, "production")
	}
	if data.MultiRegion != true {
		t.Errorf("MultiRegion = %v, want %v", data.MultiRegion, true)
	}
	if data.Ingress != string(IngressNginx) {
		t.Errorf("Ingress = %v, want %v", data.Ingress, string(IngressNginx))
	}
	if data.Registry != string(RegistryGHCR) {
		t.Errorf("Registry = %v, want %v", data.Registry, string(RegistryGHCR))
	}
	if data.Secrets == nil {
		t.Error("Secrets map should be initialized")
	}
	if len(data.Secrets) != 0 {
		t.Errorf("Secrets map should be empty, got %d entries", len(data.Secrets))
	}
}
