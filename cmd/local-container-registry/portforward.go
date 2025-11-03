package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/alexandremahdhaoui/tooling/pkg/flaterrors"
	"github.com/alexandremahdhaoui/tooling/pkg/forge"
)

var errPortForwarding = errors.New("port forwarding")

// PortForwarder manages a port-forward connection to a Kubernetes service.
type PortForwarder struct {
	config    forge.Spec
	namespace string
	localPort int
	cmd       *exec.Cmd
	started   bool
}

// NewPortForwarder creates a new port forwarder.
func NewPortForwarder(config forge.Spec, namespace string) *PortForwarder {
	return &PortForwarder{
		config:    config,
		namespace: namespace,
	}
}

// Start establishes the port-forward connection using kubectl.
// It finds an available local port and forwards it to the registry service port 5000.
func (pf *PortForwarder) Start(ctx context.Context) error {
	pf.localPort = 5000

	serviceName := fmt.Sprintf("svc/%s", Name)
	portMapping := fmt.Sprintf("%d:5000", pf.localPort)

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
		"✅ Port-forward established: 127.0.0.1:%d -> %s:5000\n",
		pf.localPort,
		serviceName,
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
				fmt.Sprintf("127.0.0.1:%d", pf.localPort),
				100*time.Millisecond,
			)
			if err == nil {
				conn.Close()
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
	return fmt.Sprintf("127.0.0.1:%d", pf.localPort)
}

// LocalPort returns the local port number.
func (pf *PortForwarder) LocalPort() int {
	return pf.localPort
}
