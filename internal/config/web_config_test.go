package config_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/config"
)

func TestWebConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.WebConfig
		wantErr bool
	}{
		{
			name:    "valid config",
			cfg:     config.WebConfig{Port: 8080, JWTSecret: "supersecret"},
			wantErr: false,
		},
		{
			name:    "port zero with secret — valid (web disabled)",
			cfg:     config.WebConfig{Port: 0, JWTSecret: "supersecret"},
			wantErr: false,
		},
		{
			name:    "port zero empty secret — valid (web disabled, secret not required)",
			cfg:     config.WebConfig{Port: 0, JWTSecret: ""},
			wantErr: false,
		},
		{
			name:    "port out of range high",
			cfg:     config.WebConfig{Port: 99999, JWTSecret: "supersecret"},
			wantErr: true,
		},
		{
			name:    "port enabled empty jwt secret",
			cfg:     config.WebConfig{Port: 8080, JWTSecret: ""},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
