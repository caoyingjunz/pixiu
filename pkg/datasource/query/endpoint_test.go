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

package query

import "testing"

func TestParseInClusterEndpoint(t *testing.T) {
	ep, err := ParseInClusterEndpoint("http://prometheus-server.pixiu-system")
	if err != nil {
		t.Fatal(err)
	}
	if ep.ServiceName != "prometheus-server" || ep.Namespace != "pixiu-system" || ep.Port != 80 {
		t.Fatalf("unexpected endpoint: %+v", ep)
	}

	ep, err = ParseInClusterEndpoint("http://prometheus-operated.monitoring.svc:9090/prom")
	if err != nil {
		t.Fatal(err)
	}
	if ep.ServiceName != "prometheus-operated" || ep.Namespace != "monitoring" || ep.Port != 9090 || ep.BasePath != "/prom" {
		t.Fatalf("unexpected svc endpoint: %+v", ep)
	}

	if _, err = ParseInClusterEndpoint("http://10.0.0.1:9090"); err == nil {
		t.Fatal("expected ip address to be rejected for internal endpoint")
	}
}

func TestParsePrometheusQueryResponseVector(t *testing.T) {
	body := []byte(`{
		"status":"success",
		"data":{
			"resultType":"vector",
			"result":[
				{"metric":{"__name__":"up","instance":"a:9100","job":"node"},"value":[1710000000,"1"]},
				{"metric":{"__name__":"up","instance":"b:9100","job":"node"},"value":[1710000000,"0"]}
			]
		}
	}`)
	samples, err := ParsePrometheusQueryResponse(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 2 {
		t.Fatalf("len(samples)=%d, want 2", len(samples))
	}
	if samples[0].Value != "1" || samples[1].Value != "0" {
		t.Fatalf("unexpected values: %+v", samples)
	}
	if samples[0].ResourceName != "instance=a:9100" {
		t.Fatalf("resourceName=%q", samples[0].ResourceName)
	}
}
