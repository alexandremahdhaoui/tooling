//go:build unit

// Copyright 2024 Alexandre Mahdhaoui
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"testing"
)

func TestNewPortForwarder_AcceptsDynamicPort(t *testing.T) {
	tests := []struct {
		name string
		port int32
	}{
		{
			name: "NodePort_30000",
			port: 30000,
		},
		{
			name: "NodePort_32767",
			port: 32767,
		},
		{
			name: "NodePort_30123",
			port: 30123,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := runConfig{}
			namespace := "testenv-lcr"

			pf := NewPortForwarder(config, namespace, tt.port)

			if pf == nil {
				t.Fatal("NewPortForwarder returned nil")
			}

			if pf.port != tt.port {
				t.Errorf("port = %d, want %d", pf.port, tt.port)
			}

			if pf.namespace != namespace {
				t.Errorf("namespace = %s, want %s", pf.namespace, namespace)
			}
		})
	}
}

func TestPortForwarder_LocalPort_ReturnsDynamicPort(t *testing.T) {
	tests := []struct {
		name string
		port int32
	}{
		{
			name: "Port_30000",
			port: 30000,
		},
		{
			name: "Port_32767",
			port: 32767,
		},
		{
			name: "Port_30456",
			port: 30456,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := runConfig{}
			pf := NewPortForwarder(config, "testenv-lcr", tt.port)

			got := pf.LocalPort()
			if got != tt.port {
				t.Errorf("LocalPort() = %d, want %d", got, tt.port)
			}
		})
	}
}

func TestPortForwarder_LocalEndpoint(t *testing.T) {
	tests := []struct {
		name             string
		port             int32
		expectedEndpoint string
	}{
		{
			name:             "Endpoint_30000",
			port:             30000,
			expectedEndpoint: "127.0.0.1:30000",
		},
		{
			name:             "Endpoint_32767",
			port:             32767,
			expectedEndpoint: "127.0.0.1:32767",
		},
		{
			name:             "Endpoint_30123",
			port:             30123,
			expectedEndpoint: "127.0.0.1:30123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := runConfig{}
			pf := NewPortForwarder(config, "testenv-lcr", tt.port)

			got := pf.LocalEndpoint()
			if got != tt.expectedEndpoint {
				t.Errorf("LocalEndpoint() = %s, want %s", got, tt.expectedEndpoint)
			}
		})
	}
}

func TestPortForwarder_GetPID_BeforeStart(t *testing.T) {
	config := runConfig{}
	pf := NewPortForwarder(config, "testenv-lcr", 30123)

	// Before Start() is called, GetPID should return 0
	pid := pf.GetPID()
	if pid != 0 {
		t.Errorf("GetPID() before Start() = %d, want 0", pid)
	}
}

// TestPortForwarder_PortMapping verifies that the port mapping format
// would use the same port on both ends. Since we can't easily test
// the actual kubectl command without running it, we verify the port
// is correctly stored and would be used in the mapping.
func TestPortForwarder_PortMapping(t *testing.T) {
	tests := []struct {
		name               string
		port               int32
		expectedLocalPort  int32
		expectedRemotePort int32
	}{
		{
			name:               "Same_port_both_ends_30123",
			port:               30123,
			expectedLocalPort:  30123,
			expectedRemotePort: 30123,
		},
		{
			name:               "Same_port_both_ends_30000",
			port:               30000,
			expectedLocalPort:  30000,
			expectedRemotePort: 30000,
		},
		{
			name:               "Same_port_both_ends_32767",
			port:               32767,
			expectedLocalPort:  32767,
			expectedRemotePort: 32767,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := runConfig{}
			pf := NewPortForwarder(config, "testenv-lcr", tt.port)

			// The port forwarder uses the same port on both ends
			// Both local and remote (service) port should be the same
			if pf.port != tt.expectedLocalPort {
				t.Errorf("local port = %d, want %d", pf.port, tt.expectedLocalPort)
			}

			// Verify LocalPort() returns the configured port
			if pf.LocalPort() != tt.expectedRemotePort {
				t.Errorf("LocalPort() = %d, want %d", pf.LocalPort(), tt.expectedRemotePort)
			}

			// Verify the port mapping would be correct format
			// In Start(), this would be: fmt.Sprintf("%d:%d", pf.port, pf.port)
			// which results in "30123:30123" format (same port on both ends)
		})
	}
}
