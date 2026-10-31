package internal

import (
	"io"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/kubereboot/kured/pkg/checkers"
	"github.com/kubereboot/kured/pkg/reboot"
	log "github.com/sirupsen/logrus"
)

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

func TestNewRebootCheckerPropagation(t *testing.T) {
	const timeout = 15 * time.Second
	got, err := NewRebootChecker("echo hi", "/unused", timeout)
	if err != nil {
		t.Fatal(err)
	}
	checker, ok := got.(*checkers.CommandChecker)
	if !ok {
		t.Fatalf("got %T, want *checkers.CommandChecker", got)
	}
	if checker.Timeout != timeout {
		t.Fatalf("Timeout = %v, want %v", checker.Timeout, timeout)
	}
	if !checker.Privileged || checker.NamespacePid != 1 {
		t.Fatalf("privileged wiring = privileged:%v pid:%d", checker.Privileged, checker.NamespacePid)
	}
	wantCommand := []string{"/usr/bin/nsenter", "-m/proc/1/ns/mnt", "--", "echo", "hi"}
	if !reflect.DeepEqual(checker.CheckCommand, wantCommand) {
		t.Fatalf("CheckCommand = %v, want %v", checker.CheckCommand, wantCommand)
	}
}

func TestNewRebootCheckerFileSelectionIgnoresTimeout(t *testing.T) {
	got, err := NewRebootChecker("", "/var/run/reboot-required", 15*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	checker, ok := got.(*checkers.FileRebootChecker)
	if !ok {
		t.Fatalf("got %T, want *checkers.FileRebootChecker", got)
	}
	if checker.FilePath != "/var/run/reboot-required" {
		t.Fatalf("FilePath = %s", checker.FilePath)
	}
}

func TestNewRebootCheckerParseError(t *testing.T) {
	_, err := NewRebootChecker(`"unterminated`, "/unused", time.Second)
	if err == nil {
		t.Fatal("expected sentinel parse error")
	}
}

func TestNewRebooterCommandPropagation(t *testing.T) {
	const timeout = 45 * time.Second
	got, err := NewRebooter("command", "echo hi", 0, timeout)
	if err != nil {
		t.Fatal(err)
	}
	rebooter, ok := got.(*reboot.CommandRebooter)
	if !ok {
		t.Fatalf("got %T, want *reboot.CommandRebooter", got)
	}
	if rebooter.Timeout != timeout {
		t.Fatalf("Timeout = %v, want %v", rebooter.Timeout, timeout)
	}
	wantCommand := []string{"/usr/bin/nsenter", "-m/proc/1/ns/mnt", "--", "echo", "hi"}
	if !reflect.DeepEqual(rebooter.RebootCommand, wantCommand) {
		t.Fatalf("RebootCommand = %v, want %v", rebooter.RebootCommand, wantCommand)
	}
}

func TestNewRebooterSignalSelectionIgnoresTimeout(t *testing.T) {
	got, err := NewRebooter("signal", "ignored", 39, 45*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	rebooter, ok := got.(*reboot.SignalRebooter)
	if !ok {
		t.Fatalf("got %T, want *reboot.SignalRebooter", got)
	}
	if rebooter.Signal != 39 {
		t.Fatalf("Signal = %d", rebooter.Signal)
	}
}

func TestNewRebooterRejectsInvalidInput(t *testing.T) {
	if _, err := NewRebooter("nope", "echo hi", 1, time.Second); err == nil {
		t.Fatal("expected invalid reboot-method error")
	}
	if _, err := NewRebooter("command", "", 1, time.Second); err == nil {
		t.Fatal("expected empty reboot command error")
	}
}
