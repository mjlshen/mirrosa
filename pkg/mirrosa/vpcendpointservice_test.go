package mirrosa

import (
	"context"
	"go.uber.org/zap/zaptest"
	"testing"
)

func TestVpcEndpointService_Validate(t *testing.T) {
	tests := []struct {
		name        string
		client      func(t *testing.T) MirrosaVpcEndpointServiceAPIClient
		privatelink bool
		wantErr     bool
	}{
		{
			name:        "non-PrivateLink",
			privatelink: false,
			wantErr:     false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			v := VpcEndpointService{
				log:         zaptest.NewLogger(t).Sugar(),
				PrivateLink: test.privatelink,
			}
			if err := v.Validate(context.TODO()); (err != nil) != test.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}
