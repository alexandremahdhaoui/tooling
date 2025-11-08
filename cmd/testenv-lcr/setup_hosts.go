package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/alexandremahdhaoui/forge/internal/util"
	"github.com/alexandremahdhaoui/forge/pkg/flaterrors"
)

const (
	hostsFilePath = "/etc/hosts"
	hostsIP       = "127.0.0.1"
)

var errManagingHostsFile = errors.New("managing /etc/hosts file")

// addHostsEntry adds an entry to /etc/hosts for the given FQDN if it doesn't already exist.
// It uses the provided prepend command (e.g., "sudo") for privileged operations.
func addHostsEntry(fqdn, prependCmd string) error {
	// Check if entry already exists
	exists, err := hostsEntryExists(fqdn)
	if err != nil {
		return flaterrors.Join(err, errManagingHostsFile)
	}

	if exists {
		_, _ = fmt.Fprintf(os.Stdout, "ℹ️ /etc/hosts entry already exists for %s\n", fqdn)
		return nil
	}

	// Add the entry
	entry := fmt.Sprintf("%s %s", hostsIP, fqdn)

	var cmd *exec.Cmd
	if prependCmd != "" {
		cmd = exec.Command(prependCmd, "sh", "-c", fmt.Sprintf("echo '%s' >> %s", entry, hostsFilePath))
	} else {
		cmd = exec.Command("sh", "-c", fmt.Sprintf("echo '%s' >> %s", entry, hostsFilePath))
	}

	if err := util.RunCmdWithStdPipes(cmd); err != nil {
		return flaterrors.Join(err, errManagingHostsFile)
	}

	_, _ = fmt.Fprintf(os.Stdout, "✅ Added /etc/hosts entry: %s\n", entry)

	return nil
}

// removeHostsEntry removes the entry from /etc/hosts for the given FQDN.
// It uses the provided prepend command (e.g., "sudo") for privileged operations.
func removeHostsEntry(fqdn, prependCmd string) error {
	// Check if entry exists
	exists, err := hostsEntryExists(fqdn)
	if err != nil {
		return flaterrors.Join(err, errManagingHostsFile)
	}

	if !exists {
		_, _ = fmt.Fprintf(os.Stdout, "ℹ️ /etc/hosts entry does not exist for %s\n", fqdn)
		return nil
	}

	// Remove the entry using sed
	sedCmd := fmt.Sprintf("sed -i '/%s/d' %s", fqdn, hostsFilePath)

	var cmd *exec.Cmd
	if prependCmd != "" {
		cmd = exec.Command(prependCmd, "sh", "-c", sedCmd)
	} else {
		cmd = exec.Command("sh", "-c", sedCmd)
	}

	if err := util.RunCmdWithStdPipes(cmd); err != nil {
		return flaterrors.Join(err, errManagingHostsFile)
	}

	_, _ = fmt.Fprintf(os.Stdout, "✅ Removed /etc/hosts entry for %s\n", fqdn)

	return nil
}

// hostsEntryExists checks if an entry for the given FQDN exists in /etc/hosts.
func hostsEntryExists(fqdn string) (bool, error) {
	file, err := os.Open(hostsFilePath)
	if err != nil {
		return false, err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Skip comments and empty lines
		if strings.HasPrefix(strings.TrimSpace(line), "#") || strings.TrimSpace(line) == "" {
			continue
		}

		// Check if the line contains the FQDN
		if strings.Contains(line, fqdn) {
			return true, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return false, err
	}

	return false, nil
}
