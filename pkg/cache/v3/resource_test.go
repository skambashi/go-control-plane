// Copyright 2018 Envoyproxy Authors
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

package cache_test

import (
	"reflect"
	"testing"

	cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
	route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
	"github.com/envoyproxy/go-control-plane/pkg/cache/types"
	"github.com/envoyproxy/go-control-plane/pkg/cache/v3"
	"github.com/envoyproxy/go-control-plane/pkg/test/resource/v3"
)

const (
	clusterName         = "cluster0"
	routeName           = "route0"
	listenerName        = "listener0"
	runtimeName         = "runtime0"
	tlsName             = "secret0"
	rootName            = "root0"
	extensionConfigName = "extensionConfig0"
)

var (
	testEndpoint        = resource.MakeEndpoint(clusterName, 8080)
	testCluster         = resource.MakeCluster(resource.Ads, clusterName)
	testRoute           = resource.MakeRoute(routeName, clusterName)
	testListener        = resource.MakeHTTPListener(resource.Ads, listenerName, 80, routeName)
	testRuntime         = resource.MakeRuntime(runtimeName)
	testSecret          = resource.MakeSecrets(tlsName, rootName)
	testExtensionConfig = resource.MakeExtensionConfig(resource.Ads, extensionConfigName, routeName)
)

func TestValidate(t *testing.T) {
	if err := testEndpoint.Validate(); err != nil {
		t.Error(err)
	}
	if err := testCluster.Validate(); err != nil {
		t.Error(err)
	}
	if err := testRoute.Validate(); err != nil {
		t.Error(err)
	}
	if err := testListener.Validate(); err != nil {
		t.Error(err)
	}
	if err := testRuntime.Validate(); err != nil {
		t.Error(err)
	}

	invalidRoute := &route.RouteConfiguration{
		Name: "test",
		VirtualHosts: []*route.VirtualHost{{
			Name:    "test",
			Domains: []string{},
		}},
	}

	if err := invalidRoute.Validate(); err == nil {
		t.Error("expected an error")
	}
	if err := invalidRoute.VirtualHosts[0].Validate(); err == nil {
		t.Error("expected an error")
	}
}

func TestGetResourceName(t *testing.T) {
	if name := cache.GetResourceName(testEndpoint); name != clusterName {
		t.Errorf("GetResourceName(%v) => got %q, want %q", testEndpoint, name, clusterName)
	}
	if name := cache.GetResourceName(testCluster); name != clusterName {
		t.Errorf("GetResourceName(%v) => got %q, want %q", testCluster, name, clusterName)
	}
	if name := cache.GetResourceName(testRoute); name != routeName {
		t.Errorf("GetResourceName(%v) => got %q, want %q", testRoute, name, routeName)
	}
	if name := cache.GetResourceName(testListener); name != listenerName {
		t.Errorf("GetResourceName(%v) => got %q, want %q", testListener, name, listenerName)
	}
	if name := cache.GetResourceName(testRuntime); name != runtimeName {
		t.Errorf("GetResourceName(%v) => got %q, want %q", testRuntime, name, runtimeName)
	}
	if name := cache.GetResourceName(nil); name != "" {
		t.Errorf("GetResourceName(nil) => got %q, want none", name)
	}
}

func TestGetResourceReferences(t *testing.T) {
	cases := []struct {
		in  types.Resource
		out map[string]bool
	}{
		{
			in:  nil,
			out: map[string]bool{},
		},
		{
			in:  testCluster,
			out: map[string]bool{clusterName: true},
		},
		{
			in: &cluster.Cluster{Name: clusterName, ClusterDiscoveryType: &cluster.Cluster_Type{Type: cluster.Cluster_EDS},
				EdsClusterConfig: &cluster.Cluster_EdsClusterConfig{ServiceName: "test"}},
			out: map[string]bool{"test": true},
		},
		{
			in:  resource.MakeHTTPListener(resource.Ads, listenerName, 80, routeName),
			out: map[string]bool{routeName: true},
		},
		{
			in:  resource.MakeTCPListener(listenerName, 80, clusterName),
			out: map[string]bool{},
		},
		{
			in:  testRoute,
			out: map[string]bool{},
		},
		{
			in:  testEndpoint,
			out: map[string]bool{},
		},
		{
			in:  testRuntime,
			out: map[string]bool{},
		},
	}
	for _, cs := range cases {
		names := cache.GetResourceReferences(cache.IndexResourcesByName([]types.ResourceWithTtl{{Resource: cs.in}}))
		if !reflect.DeepEqual(names, cs.out) {
			t.Errorf("GetResourceReferences(%v) => got %v, want %v", cs.in, names, cs.out)
		}
	}
}
