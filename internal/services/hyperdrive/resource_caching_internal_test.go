package hyperdrive

// This file deviates from the repo's external-test-package convention
// (`hyperdrive_test`, see resource_test.go) because it unit-tests
// cachingParamFromResponse directly. That helper only runs inside Update's
// fallback path, which is reachable only when the plan's `caching` is nil —
// i.e. config omits the `caching` block and state was never backfilled with
// the remote caching object (for example, `terraform apply -refresh=false`
// immediately after `terraform apply`). The terraform-plugin-testing test
// harness used in resource_test.go always refreshes state before planning,
// so that path can't be exercised end-to-end through it. Testing the
// mechanism directly here is the closest practical substitute.

import (
	"testing"

	"github.com/cloudflare/cloudflare-go/v7/hyperdrive"
)

func TestCachingParamFromResponse(t *testing.T) {
	tests := []struct {
		name            string
		response        hyperdrive.HyperdriveCaching
		wantDisabledSet bool
		wantDisabled    bool
		wantMaxAgeSet   bool
		wantMaxAge      int64
		wantSWRSet      bool
		wantSWR         int64
	}{
		{
			name:            "disabled true",
			response:        hyperdrive.HyperdriveCaching{Disabled: true, MaxAge: 0, StaleWhileRevalidate: 0},
			wantDisabledSet: true,
			wantDisabled:    true,
			wantMaxAgeSet:   false,
			wantSWRSet:      false,
		},
		{
			name:            "enabled with values",
			response:        hyperdrive.HyperdriveCaching{Disabled: false, MaxAge: 300, StaleWhileRevalidate: 30},
			wantDisabledSet: true,
			wantDisabled:    false,
			wantMaxAgeSet:   true,
			wantMaxAge:      300,
			wantSWRSet:      true,
			wantSWR:         30,
		},
		{
			name:            "enabled with zero values",
			response:        hyperdrive.HyperdriveCaching{Disabled: false, MaxAge: 0, StaleWhileRevalidate: 0},
			wantDisabledSet: true,
			wantDisabled:    false,
			wantMaxAgeSet:   false,
			wantSWRSet:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cachingParamFromResponse(&tt.response)

			if got.Disabled.Present != tt.wantDisabledSet {
				t.Fatalf("Disabled.Present = %v, want %v", got.Disabled.Present, tt.wantDisabledSet)
			}
			if tt.wantDisabledSet && got.Disabled.Value != tt.wantDisabled {
				t.Errorf("Disabled = %v, want %v", got.Disabled.Value, tt.wantDisabled)
			}

			if got.MaxAge.Present != tt.wantMaxAgeSet {
				t.Fatalf("MaxAge.Present = %v, want %v", got.MaxAge.Present, tt.wantMaxAgeSet)
			}
			if tt.wantMaxAgeSet && got.MaxAge.Value != tt.wantMaxAge {
				t.Errorf("MaxAge = %v, want %v", got.MaxAge.Value, tt.wantMaxAge)
			}

			if got.StaleWhileRevalidate.Present != tt.wantSWRSet {
				t.Fatalf("StaleWhileRevalidate.Present = %v, want %v", got.StaleWhileRevalidate.Present, tt.wantSWRSet)
			}
			if tt.wantSWRSet && got.StaleWhileRevalidate.Value != tt.wantSWR {
				t.Errorf("StaleWhileRevalidate = %v, want %v", got.StaleWhileRevalidate.Value, tt.wantSWR)
			}
		})
	}
}
