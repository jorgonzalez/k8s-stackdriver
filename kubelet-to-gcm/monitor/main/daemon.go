/*
Copyright 2017 Google Inc.

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

package main

import (
	"flag"
	"os"
	"time"

	log "github.com/golang/glog"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"

	v3 "google.golang.org/api/monitoring/v3"

	"github.com/GoogleCloudPlatform/k8s-stackdriver/kubelet-to-gcm/monitor"
	"github.com/GoogleCloudPlatform/k8s-stackdriver/kubelet-to-gcm/monitor/config"
	"github.com/GoogleCloudPlatform/k8s-stackdriver/kubelet-to-gcm/monitor/controller"
	"github.com/GoogleCloudPlatform/k8s-stackdriver/kubelet-to-gcm/monitor/kubelet"
)

const (
	scope = "https://www.googleapis.com/auth/monitoring.write"
	//testPath = "https://test-monitoring.sandbox.googleapis.com"
)

var (
	// Flags to identify the Kubelet.
	zone            = flag.String("zone", "use-gce", "The zone where this kubelet lives.")
	project         = flag.String("project", "use-gce", "The project where this kubelet's host lives.")
	cluster         = flag.String("cluster", "use-gce", "The cluster where this kubelet holds membership.")
	kubeletInstance = flag.String("kubelet-instance", "use-gce", "The instance name the kubelet resides on.")
	kubeletHost     = flag.String("kubelet-host", "use-gce", "The kubelet's host name.")
	kubeletPort     = flag.Uint("kubelet-port", 10255, "The kubelet's port.")
	ctrlPort        = flag.Uint("controller-manager-port", 10252, "The kube-controller's port.")
	// Flags to control runtime behavior.
	res         = flag.Uint("resolution", 10, "The time, in seconds, to poll the Kubelet.")
	gcmEndpoint = flag.String("gcm-endpoint", "", "The GCM endpoint to hit. Defaults to the default endpoint.")
)

func main() {
	flag.Set("logtostderr", "true") // This spoofs glog into teeing logs to stderr.

	defer log.Flush()
	flag.Parse()
	log.Infof("Invoked by %v", os.Args)

	resolution := time.Second * time.Duration(*res)

	// Initialize the configuration.
	kubeletCfg, ctrlCfg, err := config.NewConfigs(*zone, *project, *cluster, *kubeletHost, *kubeletInstance, *kubeletPort, *ctrlPort, resolution)
	if err != nil {
		log.Fatalf("Failed to initialize configuration: %v", err)
	}

	// Create objects for kubelet monitoring.
	kubeletSrc, err := kubelet.NewSource(kubeletCfg)
	if err != nil {
		log.Fatalf("Failed to create a kubelet source with config %v: %v", kubeletCfg, err)
	}
	log.Infof("The kubelet source is initialized with config %v.", kubeletCfg)

	// Create objects for kube-controller monitoring.
	ctrlSrc, err := controller.NewSource(ctrlCfg)
	if err != nil {
		log.Fatalf("Failed to create a kube-controller source with config %v: %v", ctrlCfg, err)
	}
	log.Infof("The kube-controller source is initialized with config %v.", ctrlCfg)

	// Create a GCM client.
	client, err := google.DefaultClient(context.Background(), scope)
	if err != nil {
		log.Fatalf("Failed to create a client with default context and scope %s, err: %v", scope, err)
	}
	service, err := v3.New(client)
	if err != nil {
		log.Fatalf("Failed to create a GCM v3 API service object: %v", err)
	}
	// Determine the GCE endpoint.
	if *gcmEndpoint != "" {
		service.BasePath = *gcmEndpoint
	}
	log.Infof("Using GCM endpoint %q", service.BasePath)

	for {
		go monitor.Once(kubeletSrc, service)
		go monitor.Once(ctrlSrc, service)
		time.Sleep(resolution)
	}
}
