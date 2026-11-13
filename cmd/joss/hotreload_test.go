package main

import (
	"os/exec"
	"runtime"
	"sync"
	"testing"
	"time"
)

func childCommand(t *testing.T, windowsArgs, unixArgs []string) *exec.Cmd {
	t.Helper()
	if runtime.GOOS == "windows" {
		return exec.Command(windowsArgs[0], windowsArgs[1:]...)
	}
	return exec.Command(unixArgs[0], unixArgs[1:]...)
}

func TestManagedServerChildReportsStartupFailure(t *testing.T) {
	cmd := childCommand(t,
		[]string{"cmd.exe", "/c", "exit", "7"},
		[]string{"sh", "-c", "exit 7"},
	)
	child, err := startManagedServerChild(cmd)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-child.done:
		exitErr, ok := err.(*exec.ExitError)
		if !ok || exitErr.ExitCode() != 7 {
			t.Fatalf("child exit = %v, want exit code 7", err)
		}
	case <-time.After(5 * time.Second):
		stopManagedServerChild(child)
		t.Fatal("supervisor did not observe the child process exit")
	}
}

func TestStopManagedServerChildReapsProcess(t *testing.T) {
	cmd := childCommand(t,
		[]string{"cmd.exe", "/c", "ping -n 30 127.0.0.1 > nul"},
		[]string{"sh", "-c", "sleep 30"},
	)
	child, err := startManagedServerChild(cmd)
	if err != nil {
		t.Fatal(err)
	}
	stopped := make(chan struct{})
	go func() {
		stopManagedServerChild(child)
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("stopping the supervisor left its child process running")
	}
}

func TestRequestCLIShutdownSignalsSupervisor(t *testing.T) {
	requested := make(chan struct{})
	var once sync.Once
	requestShutdown(requested, &once)
	select {
	case <-requested:
	case <-time.After(time.Second):
		t.Fatal("q shutdown request did not reach the supervisor")
	}
}
