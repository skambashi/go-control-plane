// Copyright 2020 Envoyproxy Authors
//
//   Licensed under the Apache License, Version 2.0 (the "License");
//   you may not use this file except in compliance with the License.
//   You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
//   Unless required by applicable law or agreed to in writing, software
//   distributed under the License is distributed on an "AS IS" BASIS,
//   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//   See the License for the specific language governing permissions and
//   limitations under the License.
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"flag"
	"io/ioutil"
	"os"
	"strings"
	"sync"

	v2 "github.com/envoyproxy/go-control-plane/envoy/config/bootstrap/v3"
	"github.com/envoyproxy/go-control-plane/internal/example"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/server/v3"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/ulikunitz/xz"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"
)

const (
	DEFAULT_CONFIGMAP_KEY = "no configmap key"
	DEFAULT_CONFIGMAP     = "no configmap"
)

var (
	l example.Logger

	port     uint
	basePort uint
	mode     string

	nodeID string
)

func init() {
	l = example.Logger{}

	flag.BoolVar(&l.Debug, "debug", false, "Enable xDS server debug logging")

	// The port that this xDS server listens on
	flag.UintVar(&port, "port", 18000, "xDS management server port")

	// Tell Envoy to use this Node ID
	flag.StringVar(&nodeID, "nodeID", "test-id", "Node ID")
}

// parseYaml takes in a yaml envoy config string and returns a typed version
func parseYaml(yamlString string) ([]byte, error) {
	l.Debugf("[databricks-envoy-cp] *** YAML ---> JSON ***")
	jsonString, err := yaml.YAMLToJSON([]byte(yamlString))
	if err != nil {
		return nil, err
	}
	return jsonString, nil
}

func convertJsonToPb(jsonString string) (*v2.Bootstrap, error) {
	l.Debugf("[databricks-envoy-cp] *** JSON ---> PB ***")
	config := &v2.Bootstrap{}
	r := strings.NewReader(string(jsonString))
	err := jsonpb.Unmarshal(r, config)
	// err := yaml.Unmarshal([]byte(envoyYaml), config)
	l.Errorf("Error while converting JSON -> PB: %s ", err.Error())
	if err != nil {
		return nil, err
	}
	l.Debugf("[databricks-envoy-cp] *** SUCCESS *** PB: %s", config)
	return config, nil
}

func watchForChanges(clientset *kubernetes.Clientset, configmapKey *string, configmap *string, mutex *sync.Mutex) {
	for {
		namespace := os.Getenv("CONFIG_MAP_NAMESPACE")
		watcher, err := clientset.CoreV1().ConfigMaps(namespace).Watch(context.TODO(),
			metav1.SingleObject(metav1.ObjectMeta{
				Name:      os.Getenv("CONFIG_MAP_NAME"),
				Namespace: namespace,
			}))
		if err != nil {
			panic("Unable to create watcher")
		}
		updateCurrentConfigmap(watcher.ResultChan(), configmapKey, configmap, mutex)
	}
}

func updateCurrentConfigmap(eventChannel <-chan watch.Event, configmapKey *string, configmap *string, mutex *sync.Mutex) {
	for {
		event, open := <-eventChannel
		if open {
			switch event.Type {
			case watch.Added:
				fallthrough
			case watch.Modified:
				mutex.Lock()
				l.Debugf("[databricks-envoy-cp] *** CONFIG MODIFIED ***")
				// Update our configmap
				if updatedMap, ok := event.Object.(*corev1.ConfigMap); ok {
					for key, value := range updatedMap.Data {
						l.Debugf("[databricks-envoy-cp] ConfigMap Name: %s", key)
						xzString, err := base64.StdEncoding.DecodeString(value)
						if err != nil {
							l.Errorf("Error decoding string: %s ", err.Error())
							return
						}

						r, err := xz.NewReader(bytes.NewReader(xzString))
						if err != nil {
							l.Errorf("Error decompressing string: %s ", err.Error())
							return
						}
						result, _ := ioutil.ReadAll(r)
						envoyConfigString := string(result)
						/*
							config, err := parseYaml(envoyConfigString)
							if err != nil {
								l.Errorf("Error parsing yaml string: %s ", err.Error())
								return
							}
						*/
						pb, err := convertJsonToPb(envoyConfigString)
						*configmapKey = key
						*configmap = envoyConfigString
						l.Debugf("[databricks-envoy-cp] *** PB: %s", pb)
					}
				}
				mutex.Unlock()
			case watch.Deleted:
				mutex.Lock()
				// Fall back to the default value
				*configmapKey = DEFAULT_CONFIGMAP_KEY
				*configmap = DEFAULT_CONFIGMAP
				mutex.Unlock()
			default:
				// Do nothing
			}
		} else {
			// If eventChannel is closed, it means the server has closed the connection
			return
		}
	}
}

func testClient() {
	var (
		currentConfigmapKey string
		currentConfigmap    string
		mutex               *sync.Mutex
	)
	currentConfigmapKey = DEFAULT_CONFIGMAP_KEY
	currentConfigmap = DEFAULT_CONFIGMAP

	l.Debugf("[databricks-envoy-cp] init k8s client")
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	l.Debugf("[databricks-envoy-cp] k8s client try list pods")
	// Sanity check we can list pods
	namespace := os.Getenv("CONFIG_MAP_NAMESPACE")
	pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		panic(err.Error())
	}
	l.Debugf("[databricks-envoy-cp] There are %d pods in the cluster\n", len(pods.Items))

	l.Debugf("[databricks-envoy-cp] setup watcher")
	mutex = &sync.Mutex{}
	go watchForChanges(clientset, &currentConfigmapKey, &currentConfigmap, mutex)
}

func main() {
	flag.Parse()

	l.Debugf("[databricks-envoy-cp] init")

	// Create a cache
	cache := cache.NewSnapshotCache(false, cache.IDHash{}, l)

	l.Debugf("[databricks-envoy-cp] testing k8s client")
	testClient()

	l.Debugf("[databricks-envoy-cp] running server")
	// Run the xDS server
	ctx := context.Background()
	// cb := &test.Callbacks{Debug: l.Debug}
	srv := server.NewServer(ctx, cache, nil)
	example.RunServer(ctx, srv, port)
}
