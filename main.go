// Package main provides the Demerzel process entry point.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
	"go.uber.org/dig"

	"gpt-load/internal/accountscli"
)

func main() {
	if len(os.Args) > 1 {
		os.Exit(dispatchCommand(os.Args[1:], os.Stdout, os.Stderr))
	}
	if err := runServer(); err != nil {
		rootCause := dig.RootCause(err)
		if rootCause == nil {
			rootCause = err
		}
		logrus.WithError(rootCause).Error("Demerzel stopped with an error")
		logrus.WithError(err).Debug("Demerzel startup error chain")
		os.Exit(1)
	}
}

func dispatchCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printHelp(stdout)
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		printHelp(stdout)
		return 0
	case "accounts":
		return accountscli.Run(args[1:], os.Stdin, stdout, stderr)
	case "service":
		return dispatchServiceCommand(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "Unknown command: %s\n", args[0])
		fmt.Fprintln(stderr, "Run 'demerzel help' for usage.")
		return 1
	}
}

func printHelp(output io.Writer) {
	fmt.Fprintln(output, "Demerzel - self-hosted AI API key gateway")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Usage:")
	fmt.Fprintln(output, "  demerzel                    Start the gateway")
	fmt.Fprintln(output, "  demerzel help               Display this help message")
	fmt.Fprintln(output, "  demerzel accounts           Manage API-key accounts")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Windows Service Commands:")
	fmt.Fprintln(output, "  demerzel service start      Start the Windows service")
	fmt.Fprintln(output, "  demerzel service stop       Stop the Windows service")
	fmt.Fprintln(output, "  demerzel service restart    Restart the Windows service")
	fmt.Fprintln(output, "  demerzel service status     Display the Windows service status")
}

func runServer() error {
	runtime, err := buildManagedRuntime()
	if err != nil {
		return err
	}
	quit := make(chan os.Signal, 2)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(quit)

	if err := runtime.start(); err != nil {
		return err
	}

	var serveErr error
	var firstSignal os.Signal
	select {
	case firstSignal = <-quit:
		logrus.WithFields(logrus.Fields{
			"event":  "shutdown.requested",
			"signal": firstSignal.String(),
		}).Info("shutdown requested by signal")
	case serveErr = <-runtime.serveErrors():
	}

	force := make(chan struct{})
	stopResult := make(chan error, 1)
	go func() { stopResult <- runtime.stop(force) }()
	forceReason := "signal_during_shutdown"
	if firstSignal != nil {
		forceReason = "second_signal"
	}
	select {
	case secondSignal := <-quit:
		logrus.WithFields(logrus.Fields{
			"event":  "shutdown.force",
			"signal": secondSignal.String(),
			"reason": forceReason,
		}).Warn("additional shutdown signal received; forcing shutdown")
		close(force)
		if err := <-stopResult; err != nil {
			return errors.Join(serveErr, fmt.Errorf("stop application: %w", err))
		}
		return serveErr
	case err := <-stopResult:
		if err != nil {
			return errors.Join(serveErr, fmt.Errorf("stop application: %w", err))
		}
		return serveErr
	}
}
