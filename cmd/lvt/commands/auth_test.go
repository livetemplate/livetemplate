package commands

import (
	"testing"
)

func TestAuth_Flags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "default - all features enabled",
			args:    []string{},
			wantErr: false,
		},
		{
			name:    "disable magic link",
			args:    []string{"--no-magic-link"},
			wantErr: false,
		},
		{
			name:    "disable password",
			args:    []string{"--no-password"},
			wantErr: false,
		},
		{
			name:    "both password and magic-link disabled",
			args:    []string{"--no-password", "--no-magic-link"},
			wantErr: true,
			errMsg:  "at least one authentication method (password or magic-link) must be enabled",
		},
		{
			name:    "disable email confirmation",
			args:    []string{"--no-email-confirm"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Auth(tt.args)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if err.Error() != tt.errMsg {
					t.Errorf("expected error %q, got %q", tt.errMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
