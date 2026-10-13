package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// TestSecondSignalEndsTheProcess runs this test binary again as a child that
// waits in SignalContext and then ignores its context for ten seconds, as a
// long stop does; the first SIGTERM ends the context, the second the child.
func TestSecondSignalEndsTheProcess(t *testing.T) {
	if os.Getenv("KCR_SIGNAL_CHILD") == "1" {
		ctx, stop := SignalContext(context.Background(), syscall.SIGTERM)
		_ = stop
		_, _ = fmt.Fprintln(os.Stdout, "ready")
		<-ctx.Done()
		_, _ = fmt.Fprintln(os.Stdout, "stopping")
		time.Sleep(10 * time.Second)
		os.Exit(0)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestSecondSignalEndsTheProcess$")
	cmd.Env = append(os.Environ(), "KCR_SIGNAL_CHILD=1")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	lines := bufio.NewScanner(out)
	expect := func(want string) {
		t.Helper()
		if !lines.Scan() || lines.Text() != want {
			t.Fatalf("child said %q, want %q", lines.Text(), want)
		}
	}
	expect("ready")
	_ = cmd.Process.Signal(syscall.SIGTERM)
	expect("stopping")
	begun := time.Now()
	_ = cmd.Process.Signal(syscall.SIGTERM)
	err = cmd.Wait()
	if took := time.Since(begun); took > 5*time.Second {
		t.Fatalf("the child took %v after the second signal", took)
	}
	status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() || status.Signal() != syscall.SIGTERM {
		t.Fatalf("child ended with %v, want death by SIGTERM", err)
	}
}
