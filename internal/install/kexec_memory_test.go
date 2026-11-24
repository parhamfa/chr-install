package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func labMeminfo(availableKB, cmaFreeKB uint64) []byte {
	text := fmt.Sprintf("MemTotal:        1000000 kB\nMemFree:          900000 kB\nMemAvailable:    %d kB\n", availableKB)
	if cmaFreeKB > 0 {
		text += fmt.Sprintf("CmaTotal:              0 kB\nCmaFree:         %d kB\n", cmaFreeKB)
	}
	return []byte(text)
}

func TestCheckKexecMemory(t *testing.T) {
	const mib = 1024 * 1024
	kernel, initrd := uint64(16*mib), uint64(76*mib)
	tests := []struct {
		name    string
		meminfo []byte
		wantErr string
	}{
		// MemAvailable and CmaFree measured right before kexec on Ubuntu 26.04
		// 7.0 kernels in QEMU; every 1 GiB load (12 of 12) was OOM-killed.
		{"1 GiB host with handover scratch", labMeminfo(757260, 600064), "kho=off"},
		{"1.1 GiB host with handover scratch", labMeminfo(887388, 608256), ""},
		{"1.25 GiB host with handover scratch", labMeminfo(968344, 598016), ""},
		{"2 GiB host with handover scratch", labMeminfo(1900000, 659456), ""},
		{"host without CMA", labMeminfo(700000, 0), ""},
		{"busy host without CMA", labMeminfo(200000, 0), "only 195 MiB is usable"},
		{"scratch larger than available", labMeminfo(500000, 600064), "only 0 MiB is usable"},
		{"kernel without MemAvailable", []byte("MemTotal: 1000000 kB\nMemFree: 10 kB\n"), ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := checkKexecMemory(test.meminfo, kernel, initrd)
			if test.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected refusal: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("got %v, want error containing %q", err, test.wantErr)
			}
			if !strings.Contains(err.Error(), "needs 248 MiB") {
				t.Fatalf("error does not state the requirement: %v", err)
			}
		})
	}
}

func TestCheckKexecMemoryMentionsHandoverOnlyWithScratch(t *testing.T) {
	err := checkKexecMemory(labMeminfo(100000, 0), 16<<20, 76<<20)
	if err == nil || strings.Contains(err.Error(), "kho=off") {
		t.Fatalf("host without scratch should not be told to use kho=off: %v", err)
	}
}

func TestCheckKexecMemoryRejectsOverflow(t *testing.T) {
	if err := checkKexecMemory(labMeminfo(1, 0), ^uint64(0)/2, 1); err == nil || !strings.Contains(err.Error(), "overflow") {
		t.Fatalf("expected overflow error, got %v", err)
	}
}

func TestKexecMemoryReadyReadsHostFiles(t *testing.T) {
	directory := t.TempDir()
	kernelPath := filepath.Join(directory, "vmlinuz")
	initrdPath := filepath.Join(directory, "initrd.img")
	for path, size := range map[string]int64{kernelPath: 16 << 20, initrdPath: 76 << 20} {
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := file.Truncate(size); err != nil {
			t.Fatal(err)
		}
		file.Close()
	}
	previous := meminfoPath
	t.Cleanup(func() { meminfoPath = previous })
	meminfoPath = filepath.Join(directory, "meminfo")
	stage := Stage{KernelPath: kernelPath, InitrdPath: initrdPath}

	if err := os.WriteFile(meminfoPath, labMeminfo(757260, 600064), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := kexecMemoryReady(stage); err == nil {
		t.Fatal("1 GiB host with handover scratch was allowed to kexec")
	}
	if err := os.WriteFile(meminfoPath, labMeminfo(887388, 608256), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := kexecMemoryReady(stage); err != nil {
		t.Fatalf("1.1 GiB host was refused: %v", err)
	}
	meminfoPath = filepath.Join(directory, "missing")
	if err := kexecMemoryReady(stage); err != nil {
		t.Fatalf("unreadable meminfo should leave kexec to decide: %v", err)
	}
}
