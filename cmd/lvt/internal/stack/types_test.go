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
