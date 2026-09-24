package main

import (
	"bytes"
	"runtime"
	"strings"
	"testing"
)

func TestPrintHelpUsesProductNameAndAccountsCommand(t *testing.T) {
	var output bytes.Buffer
	printHelp(&output)

	help := output.String()
	for _, required := range []string{"demerzel help", "demerzel accounts", "Demerzel"} {
		if !strings.Contains(help, required) {
			t.Fatalf("help does not document %q", required)
		}
	}
	if strings.Contains(help, "migrate-keys") {
		t.Fatal("help still exposes the deferred key migration placeholder")
	}
}

func TestPrintHelpDocumentsOnlyPublicWindowsServiceManagement(t *testing.T) {
	var output bytes.Buffer
	printHelp(&output)

	help := output.String()
	for _, required := range []string{
		"service start",
		"service stop",
		"service restart",
		"service status",
		"Windows",
	} {
		if !strings.Contains(help, required) {
			t.Fatalf("help does not document %q", required)
		}
	}
	for _, internal := range []string{"service install", "service uninstall"} {
		if strings.Contains(help, internal) {
			t.Fatalf("help exposes internal command %q", internal)
		}
	}
}

func TestDispatchCommandRoutesAccountHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := dispatchCommand([]string{"accounts", "--help"}, &stdout, &stderr)
	if exitCode != 0 || stderr.Len() != 0 {
		t.Fatal("accounts help did not exit successfully")
	}
	for _, required := range []string{"accounts list", "accounts add", "accounts label", "accounts remove"} {
		if !strings.Contains(stdout.String(), required) {
			t.Fatalf("account help does not document %q", required)
		}
	}
}

func TestDispatchCommandRemovesKeyMigrationPlaceholder(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := dispatchCommand([]string{"migrate-keys"}, &stdout, &stderr)
	if exitCode == 0 || !strings.Contains(stderr.String(), "Unknown command") ||
		strings.Contains(stderr.String(), "later release") {
		t.Fatal("key migration placeholder remains a deferred command")
	}
}

func TestDispatchCommandRecognizesWindowsServiceNamespace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("non-Windows boundary test")
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := dispatchCommand([]string{"service", "status"}, &stdout, &stderr)

	if exitCode == 0 {
		t.Fatal("dispatchCommand(service status) exit code = 0 on a non-Windows host")
	}
	if strings.Contains(stderr.String(), "Unknown command") ||
		!strings.Contains(stderr.String(), "Windows") {
		t.Fatalf("stderr does not identify the Windows service boundary: %s", stderr.String())
	}
}
