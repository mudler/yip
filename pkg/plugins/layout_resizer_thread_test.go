package plugins

import (
	"errors"
	"os"
	"runtime"
	"sync"
	"testing"

	"golang.org/x/sys/unix"
)

// mountNamespace returns the mount namespace of the OS thread the calling
// goroutine runs on right now.
func mountNamespace(t *testing.T) string {
	t.Helper()
	ns, err := os.Readlink("/proc/thread-self/ns/mnt")
	if err != nil {
		t.Skipf("cannot read the thread mount namespace: %v", err)
	}
	return ns
}

func TestRunOnDedicatedThreadUsesAnotherThread(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	caller := unix.Gettid()
	var inner int
	if err := runOnDedicatedThread(func() error {
		inner = unix.Gettid()
		return nil
	}); err != nil {
		t.Fatalf("runOnDedicatedThread returned %v, want nil", err)
	}
	if inner == caller {
		t.Fatalf("fn ran on the caller's thread %d; an unshare there would outlive the call", caller)
	}
}

func TestRunOnDedicatedThreadPropagatesError(t *testing.T) {
	want := errors.New("boom")
	if got := runOnDedicatedThread(func() error { return want }); !errors.Is(got, want) {
		t.Fatalf("runOnDedicatedThread returned %v, want %v", got, want)
	}
}

// TestRunOnDedicatedThreadRetiresTheThread is the regression test for the bug
// this helper exists for. Growing a filesystem unshares the mount namespace,
// and before the fix it did so on whatever OS thread the goroutine happened to
// sit on, with no runtime.LockOSThread. The runtime then handed that thread to
// unrelated goroutines, which saw a stale mount tree: in immucore's boot DAG
// the bind-mount step stopped seeing the partitions the earlier steps had just
// mounted (kairos-io/kairos#4837).
//
// The unshare itself needs CAP_SYS_ADMIN, so the property under test here is
// the one that makes it safe and that any user can check: the thread fn ran on
// is retired, and no later goroutine is scheduled onto it.
func TestRunOnDedicatedThreadRetiresTheThread(t *testing.T) {
	var used int
	if err := runOnDedicatedThread(func() error {
		used = unix.Gettid()
		return nil
	}); err != nil {
		t.Fatalf("runOnDedicatedThread returned %v, want nil", err)
	}

	// Ask for far more threads than the helper could have left behind, so a
	// pooled thread would be picked up with near certainty.
	const probes = 256
	var wg sync.WaitGroup
	var mu sync.Mutex
	start := make(chan struct{})
	hit := false
	for i := 0; i < probes; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
			<-start
			if unix.Gettid() == used {
				mu.Lock()
				hit = true
				mu.Unlock()
			}
		}()
	}
	close(start)
	wg.Wait()

	if hit {
		t.Fatalf("a later goroutine ran on thread %d, the one fn mutated", used)
	}
}

// TestRunOnDedicatedThreadContainsUnshare checks the real thing where the test
// runs with enough privilege: after fn unshares, the caller's mount namespace
// is untouched.
func TestRunOnDedicatedThreadContainsUnshare(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("unsharing a mount namespace needs CAP_SYS_ADMIN")
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	before := mountNamespace(t)

	var inside string
	if err := runOnDedicatedThread(func() error {
		if err := unix.Unshare(unix.CLONE_NEWNS); err != nil {
			return err
		}
		ns, err := os.Readlink("/proc/thread-self/ns/mnt")
		if err != nil {
			return err
		}
		inside = ns
		return nil
	}); err != nil {
		t.Skipf("could not unshare in this environment: %v", err)
	}

	if inside == before {
		t.Fatalf("the unshare did not take effect, namespace stayed %s", inside)
	}
	if got := mountNamespace(t); got != before {
		t.Fatalf("caller namespace changed from %s to %s", before, got)
	}
}
