package application

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeIPAllowlist(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "normalizes delimiters and duplicates",
			input: " 192.0.2.1,192.0.2.1\n2001:db8::1 192.0.2.0/24 ",
			want:  "192.0.2.1\n2001:db8::1\n192.0.2.0/24",
		},
		{name: "allows an empty list", input: " , \n ", want: ""},
		{name: "rejects malformed entries", input: "192.0.2.1,not-an-ip", wantErr: true},
		{name: "rejects oversized list", input: strings.Repeat("a", maxSourceIPAllowlistLength+1), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeIPAllowlist(tt.input)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalid) {
					t.Fatalf("normalizeIPAllowlist() error = %v, want %v", err, ErrInvalid)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeIPAllowlist() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("normalizeIPAllowlist() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSourceInputLengthValidation(t *testing.T) {
	service := &Service{}

	if _, err := service.CreateSource(CreateSourceInput{
		Name:        "valid",
		Description: strings.Repeat("x", maxSourceDescriptionLength+1),
		ProductID:   1,
		SKUID:       1,
	}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("CreateSource() description error = %v, want %v", err, ErrInvalid)
	}

	if _, err := service.UpdateSource(1, UpdateSourceInput{
		Name:        "valid",
		Description: "valid",
		ProductID:   1,
		SKUID:       1,
		IPAllowlist: "invalid-cidr",
	}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("UpdateSource() allowlist error = %v, want %v", err, ErrInvalid)
	}
}
