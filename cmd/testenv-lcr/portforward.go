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
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/alexandremahdhaoui/forge/pkg/flaterrors"
)

var errPortForwarding = errors.New("port forwarding")

// PortForwarder manages a port-forward connection to a Kubernetes service.
type PortForwarder struct {
	config    runConfig
	namespace string
	port      int32 // dynamic port used on both ends (local and service)
	cmd       *exec.Cmd
	started   bool
}

// NewPortForwarder creates a new port forwarder with the specified dynamic port.
// The port is used on both the local side and the service side (e.g., 30123:30123).
func NewPortForwarder(config runConfig, namespace string, port int32) *PortForwarder {
	return &PortForwarder{
		config:    config,
		namespace: namespace,
		port:      port,
	}
}

// Start establishes the port-forward connection using kubectl.
// It uses the same dynamic port on both ends (e.g., 30123:30123).
func (pf *PortForwarder) Start(ctx context.Context) error {
	serviceName := fmt.Sprintf("svc/%s", Name)
	portMapping := fmt.Sprintf("%d:%d", pf.port, pf.port) // same port on both ends

	// Create kubectl port-forward command
	pf.cmd = exec.Command(
		"kubectl",
		"port-forward",
		"-n", pf.namespace,
		serviceName,
		portMapping,
	)

	// Set KUBECONFIG environment variable
	pf.cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("KUBECONFIG=%s", pf.config.Kindenv.KubeconfigPath),
	)

	// Capture stdout and stderr for debugging
	pf.cmd.Stdout = os.Stdout
	pf.cmd.Stderr = os.Stderr

	// Start the command
	if err := pf.cmd.Start(); err != nil {
		return flaterrors.Join(err, errPortForwarding)
	}

	pf.started = true

	// Wait for port to be ready
	if err := pf.waitForReady(ctx); err != nil {
		pf.Stop()
		return flaterrors.Join(err, errPortForwarding)
	}

	_, _ = fmt.Fprintf(
		os.Stdout,
		"Port-forward established: 127.0.0.1:%d -> %s:%d\n",
		pf.port,
		serviceName,
		pf.port,
	)

	return nil
}

// waitForReady waits for the port-forward to be ready by attempting to connect to the local port.
func (pf *PortForwarder) waitForReady(ctx context.Context) error {
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return errors.New("timeout waiting for port-forward to be ready")
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Try to connect to the local port
			conn, err := net.DialTimeout(
				"tcp",
				fmt.Sprintf("127.0.0.1:%d", pf.port),
				100*time.Millisecond,
			)
			if err == nil {
				_ = conn.Close()
				return nil
			}
		}
	}
}

// Stop closes the port-forward connection.
func (pf *PortForwarder) Stop() {
	if pf.started && pf.cmd != nil && pf.cmd.Process != nil {
		_ = pf.cmd.Process.Kill()
		_ = pf.cmd.Wait()
		_, _ = fmt.Fprintf(os.Stdout, "✅ Port-forward closed\n")
	}
}

// LocalEndpoint returns the local endpoint (127.0.0.1:port) to connect to.
func (pf *PortForwarder) LocalEndpoint() string {
	return fmt.Sprintf("127.0.0.1:%d", pf.port)
}

// LocalPort returns the local port number.
func (pf *PortForwarder) LocalPort() int32 {
	return pf.port
}

// GetPID returns the process ID of the port-forward process, or 0 if not started.
func (pf *PortForwarder) GetPID() int {
	if pf.cmd != nil && pf.cmd.Process != nil {
		return pf.cmd.Process.Pid
	}
	return 0
}
