/*
Copyright 2024 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cluster

import "testing"

func TestEnsureHTTPSGatewayBase(t *testing.T) {
	tests := []struct {
		name      string
		base      string
		httpsPort int
		want      string
		wantErr   bool
	}{
		{
			name:      "https without port adds listen port",
			base:      "https://console.cloud.pixiuio.com",
			httpsPort: 8443,
			want:      "https://console.cloud.pixiuio.com:8443",
		},
		{
			name:      "https with explicit port kept",
			base:      "https://console.cloud.pixiuio.com:443",
			httpsPort: 8443,
			want:      "https://console.cloud.pixiuio.com:443",
		},
		{
			name:      "http rewritten to https with listen port",
			base:      "http://console.cloud.pixiuio.com:8091",
			httpsPort: 8443,
			want:      "https://console.cloud.pixiuio.com:8443",
		},
		{
			name:      "trailing slash trimmed",
			base:      "https://console.cloud.pixiuio.com/",
			httpsPort: 8443,
			want:      "https://console.cloud.pixiuio.com:8443",
		},
		{
			name:    "empty rejected",
			base:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ensureHTTPSGatewayBase(tt.base, tt.httpsPort)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
