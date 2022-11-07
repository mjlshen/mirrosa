package ocm

import (
	"net/url"
	"testing"
)

func TestGenerateBackplaneUrl(t *testing.T) {
	tests := []struct {
		url       string
		expected  string
		expectErr bool
	}{
		{
			url:       "https://api.hiveshard.slug.p1.openshiftapps.com:6443",
			expected:  "https://api-backplane.apps.hiveshard.slug.p1.openshiftapps.com",
			expectErr: false,
		},
	}

	for _, test := range tests {
		input, err := url.Parse(test.url)
		if err != nil {
			t.Fatalf("failed to parse url %s", test.url)
		}

		actual, err := GenerateBackplaneUrl(input)
		if !test.expectErr && err != nil {
			t.Fatalf("expected no error for %s, got %s", test.url, err)
		} else {
			if actual.String() != test.expected {
				t.Fatalf("expected: %s, actual: %s", test.expected, actual)
			}
		}
	}
}
