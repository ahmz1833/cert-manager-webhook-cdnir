package main

import (
	"testing"
)

func TestExtractRecordName(t *testing.T) {
	tests := []struct {
		name     string
		fqdn     string
		zone     string
		expected string
	}{
		{
			name:     "root apex domain",
			fqdn:     "example.ir.",
			zone:     "example.ir.",
			expected: "@",
		},
		{
			name:     "subdomain acme challenge",
			fqdn:     "_acme-challenge.example.ir.",
			zone:     "example.ir.",
			expected: "_acme-challenge",
		},
		{
			name:     "nested subdomain acme challenge",
			fqdn:     "_acme-challenge.api.example.ir.",
			zone:     "example.ir.",
			expected: "_acme-challenge.api",
		},
		{
			name:     "deeply nested subdomain acme challenge",
			fqdn:     "_acme-challenge.foo.bar.example.ir.",
			zone:     "example.ir.",
			expected: "_acme-challenge.foo.bar",
		},
		{
			name:     "names without trailing dots",
			fqdn:     "_acme-challenge.api.example.ir",
			zone:     "example.ir",
			expected: "_acme-challenge.api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := extractRecordName(tt.fqdn, tt.zone)
			if actual != tt.expected {
				t.Errorf("extractRecordName(%q, %q) = %q; expected %q", tt.fqdn, tt.zone, actual, tt.expected)
			}
		})
	}
}
