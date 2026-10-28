/*
Copyright 2026 The Pixiu Authors.

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

package datasource

import "testing"

func TestParseInClusterEndpointPort(t *testing.T) {
	valid := []struct {
		rawURL  string
		service string
		ns      string
		port    int
	}{
		{"http://prometheus-server.pixiu-system", "prometheus-server", "pixiu-system", 80},
		{"https://prometheus-server.pixiu-system", "prometheus-server", "pixiu-system", 443},
		{"http://prometheus-server.pixiu-system.svc.cluster.local:9090/prometheus", "prometheus-server", "pixiu-system", 9090},
		{"http://prometheus-server.pixiu-system:65535", "prometheus-server", "pixiu-system", 65535},
	}
	for _, tc := range valid {
		ep, err := ParseInClusterEndpoint(tc.rawURL)
		if err != nil {
			t.Fatalf("ParseInClusterEndpoint(%q) unexpected error: %v", tc.rawURL, err)
		}
		if ep.ServiceName != tc.service || ep.Namespace != tc.ns || ep.Port != tc.port {
			t.Fatalf("ParseInClusterEndpoint(%q) = %+v, want service=%q namespace=%q port=%d", tc.rawURL, ep, tc.service, tc.ns, tc.port)
		}
	}

	invalid := []string{
		"http://prometheus-server.pixiu-system:0",
		"http://prometheus-server.pixiu-system:65536",
		"http://prometheus-server.pixiu-system:99999",
	}
	for _, rawURL := range invalid {
		if ep, err := ParseInClusterEndpoint(rawURL); err == nil {
			t.Fatalf("ParseInClusterEndpoint(%q) = %+v, want error", rawURL, ep)
		}
	}
}
