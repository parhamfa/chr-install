package install

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// kexecLoadSpare is the kernel memory kept free for the rest of the host
// while kexec loads the writer.
const kexecLoadSpare = 64 * 1024 * 1024

var meminfoPath = "/proc/meminfo"

// kexecMemoryReady reports whether the running host can load the staged
// writer with kexec without exhausting kernel memory.
//
// kexec_file_load reads the kernel and initramfs into kernel buffers and then
// copies them into the segments it will boot, so both copies exist at once,
// and none of it can live in movable-only memory. Ubuntu 26.04 kernels
// reserve kexec handover (KHO) scratch areas at boot that admit only movable
// pages, and the kernel counts their free pages as CmaFree. On a 1 GiB host
// those areas hold about 600 MiB, and loading a 76 MiB writer was
// OOM-killed every time. Measured in the lab: 153 MiB usable failed, 272 MiB
// and more succeeded.
func kexecMemoryReady(stage Stage) error {
	meminfo, err := os.ReadFile(meminfoPath)
	if err != nil {
		return nil
	}
	kernel, err := os.Stat(stage.KernelPath)
	if err != nil {
		return err
	}
	initrd, err := os.Stat(stage.InitrdPath)
	if err != nil {
		return err
	}
	return checkKexecMemory(meminfo, uint64(kernel.Size()), uint64(initrd.Size()))
}

func checkKexecMemory(meminfo []byte, kernelBytes, initrdBytes uint64) error {
	available, cmaFree, ok := parseKexecMeminfo(meminfo)
	if !ok {
		return nil
	}
	const limit = ^uint64(0)
	if kernelBytes > limit/4 || initrdBytes > (limit-kexecLoadSpare)/2-kernelBytes {
		return fmt.Errorf("kexec memory estimate overflowed")
	}
	required := 2*(kernelBytes+initrdBytes) + kexecLoadSpare
	usable := uint64(0)
	if available > cmaFree {
		usable = available - cmaFree
	}
	if usable >= required {
		return nil
	}
	message := fmt.Sprintf("loading the writer with kexec needs %d MiB of kernel memory; only %d MiB is usable", mebibytes(required), mebibytes(usable))
	if cmaFree > 0 {
		message += fmt.Sprintf(" because %d MiB of free memory is reserved for movable pages (kexec handover on Ubuntu 26.04 kernels; booting the host once with kho=off releases it)", mebibytes(cmaFree))
	}
	return fmt.Errorf("%s", message)
}

func parseKexecMeminfo(data []byte) (available, cmaFree uint64, ok bool) {
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil || value > ^uint64(0)/1024 {
			continue
		}
		switch fields[0] {
		case "MemAvailable:":
			available, ok = value*1024, true
		case "CmaFree:":
			cmaFree = value * 1024
		}
	}
	return available, cmaFree, ok
}

func mebibytes(value uint64) uint64 {
	return value / (1024 * 1024)
}
