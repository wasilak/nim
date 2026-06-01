package providers

import (
	"testing"
)

func TestParseMasListLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantID   string
		wantName string
		wantVer  string
	}{
		{
			name:     "standard line with version",
			line:     "497799835 Xcode (16.2)",
			wantID:   "497799835",
			wantName: "Xcode",
			wantVer:  "16.2",
		},
		{
			name:     "multi-word app name",
			line:     "1091189122 Bear Notes (2.0)",
			wantID:   "1091189122",
			wantName: "Bear Notes",
			wantVer:  "2.0",
		},
		{
			name:     "no version parentheses",
			line:     "128919828 Things",
			wantID:   "128919828",
			wantName: "Things",
			wantVer:  "",
		},
		{
			name:     "empty line",
			line:     "",
			wantID:   "",
			wantName: "",
			wantVer:  "",
		},
		{
			name:     "version with build number",
			line:     "497799835 Xcode (16.2.1)",
			wantID:   "497799835",
			wantName: "Xcode",
			wantVer:  "16.2.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseMasListLine(tt.line)
			if got.id != tt.wantID {
				t.Errorf("id = %q, want %q", got.id, tt.wantID)
			}
			if got.name != tt.wantName {
				t.Errorf("name = %q, want %q", got.name, tt.wantName)
			}
			if got.version != tt.wantVer {
				t.Errorf("version = %q, want %q", got.version, tt.wantVer)
			}
		})
	}
}

func TestValidateAppID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"497799835", true},
		{"0", true},
		{"", false},
		{"abc", false},
		{"497799835abc", false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			if got := validateAppID(tt.id); got != tt.want {
				t.Errorf("validateAppID(%q) = %v, want %v", tt.id, got, tt.want)
			}
		})
	}
}