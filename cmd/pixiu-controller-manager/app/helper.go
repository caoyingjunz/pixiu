/*
Copyright 2021.

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

package app

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	clientgokubescheme "k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

const (
	defaultReadinessEndpoint = "/readyz"
	defaultLivenessEndpoint  = "/healthz"
	defaultMetricsEndpoint   = "/metrics"
)

// WaitForAPIServer waits for the API Server's /healthz endpoint to report "ok" with timeout.
func WaitForAPIServer(client clientset.Interface, timeout time.Duration) error {
	var lastErr error

	err := wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		healthStatus := 0
		result := client.Discovery().RESTClient().Get().AbsPath("/healthz").Do(context.TODO()).StatusCode(&healthStatus)
		if result.Error() != nil {
			lastErr = fmt.Errorf("failed to get apiserver /healthz status: %v", result.Error())
			return false, nil
		}
		if healthStatus != http.StatusOK {
			content, _ := result.Raw()
			lastErr = fmt.Errorf("APIServer isn't healthy: %v", string(content))
			klog.Warningf("APIServer isn't healthy yet: %v. Waiting a little while.", string(content))
			return false, nil
		}

		return true, nil
	})

	if err != nil {
		return fmt.Errorf("%v: %v", err, lastErr)
	}

	return nil
}

func StartHealthzServer(healthzHost string, healthzPort string) {
	var healthzHandler *healthz.Handler
	healthzHandler = &healthz.Handler{Checks: map[string]healthz.Checker{}}
	healthzHandler.Checks["ping"] = healthz.Ping

	mux := http.NewServeMux()
	server := http.Server{
		Handler: mux,
	}
	mux.Handle(defaultLivenessEndpoint, http.StripPrefix(defaultLivenessEndpoint, healthzHandler))
	mux.Handle(defaultLivenessEndpoint+"/", http.StripPrefix(defaultLivenessEndpoint, healthzHandler))

	healthProbeListener, err := net.Listen("tcp",healthzHost+":"+healthzPort)
	if err != nil {
		klog.Fatalf("error starting healthserver: %v", err)
	}

	klog.Infof("Starting Healthz Server...")
	klog.Fatal(server.Serve(healthProbeListener))
}

func CreateRecorder(kubeClient clientset.Interface, userAgent string) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	return eventBroadcaster.NewRecorder(clientgokubescheme.Scheme, v1.EventSource{Component: userAgent})
}
