# chr-install

[![CI](https://github.com/parhamfa/chr-install/actions/workflows/ci.yml/badge.svg)](https://github.com/parhamfa/chr-install/actions/workflows/ci.yml)
[![Full CHR/QEMU test](https://github.com/parhamfa/chr-install/actions/workflows/qemu.yml/badge.svg)](https://github.com/parhamfa/chr-install/actions/workflows/qemu.yml)
[![Latest release](https://img.shields.io/github/v/release/parhamfa/chr-install)](https://github.com/parhamfa/chr-install/releases/latest)
[![MIT License](https://img.shields.io/github/license/parhamfa/chr-install)](LICENSE)

`chr-install` is an interactive, fail-closed installer that replaces a supported Debian or Ubuntu server with MikroTik CHR while preserving a reviewed Layer-3 network configuration.

It is intentionally not a RouterOS hardening tool. It does not create passwords or users, change firewall policy, restrict services, configure licensing, or make unrelated RouterOS changes.

> [!CAUTION]
> This installer erases the complete disk that currently contains Linux. A provider console or rescue environment is strongly recommended. No program can recover a server from incorrect provider-side routing, unsupported virtual hardware, or unavailable recovery controls.

## Supported systems

- AMD64 only
- Debian 12 and 13
- Ubuntu 22.04, 24.04, and 26.04 LTS
- Firmware modes validated for the exact RouterOS release (7.21.5 and 7.23.5: legacy BIOS and installer-prepared UEFI)
- One unambiguous local boot disk
- A reboot-observable serial or WWN for the target when additional physical disks are visible
- One Ethernet uplink with a single routing policy
- IPv4 DHCP or static addressing
- Same-uplink DHCP routes whose complete forwarding attributes match the selected DHCP default route
- IPv6 SLAAC, DHCPv6, static addressing, or combinations of those modes
- Same-subnet and provider-routed/off-link gateways
- Rescue-system direct writing, or a RAM-backed pre-root writer from normal Linux

The preflight rejects RAID, multipath, ambiguous rescue disks, multiple uplinks/default routes, policy routing, VLANs, bonds, bridges, PPP, and other layouts that v1 cannot translate credibly.

V1 blocks a UEFI-booted Linux host unless that exact long-term CHR release has passed the installer's UEFI preparation and boot matrix. The official CHR 7.21.5 and 7.23.5 images place `BOOTX64.EFI` and their boot maps on an ext2 partition that standard UEFI firmware cannot read. On a validated UEFI host, the installer copies those verified files into a deterministic FAT16 filesystem in the same existing 32 MiB EFI-designated partition. It does not add or move partitions, and BIOS hosts retain the official ext2 boot partition unchanged.

## Run preflight first

### GitHub bootstrap

If the server can reach GitHub, run:

```bash
sudo bash -c "$(curl -fsSL https://raw.githubusercontent.com/parhamfa/chr-install/main/installer.sh)" -- --preflight
```

### Manual upload

If the server cannot reach GitHub, download the latest [Linux AMD64 binary](https://github.com/parhamfa/chr-install/releases/latest/download/chr-install-linux-amd64) on another computer. Before transferring it, verify it against the published [SHA-256 checksum](https://github.com/parhamfa/chr-install/releases/latest/download/chr-install-linux-amd64.sha256) with `sha256sum --check chr-install-linux-amd64.sha256` or an equivalent tool.

Upload only `chr-install-linux-amd64` to an executable filesystem on the server using SCP, SFTP, or the provider's file-transfer facility. From the directory containing the uploaded binary, run:

```bash
chmod 0700 chr-install-linux-amd64 && sudo ./chr-install-linux-amd64 --preflight
```

This method makes no GitHub request from the server. The installer still requires outbound HTTPS access to MikroTik to discover and download the RouterOS release, CHR image, and official checksum.

Preflight does not modify the server. It reports the resolved target disk, installation path, current addresses and routes, DNS, MTU, DHCP availability, current RouterOS long-term release, and any blockers.

## Install

With the GitHub bootstrap:

```bash
sudo bash -c "$(curl -fsSL https://raw.githubusercontent.com/parhamfa/chr-install/main/installer.sh)"
```

Or, with the manually uploaded binary:

```bash
chmod 0700 chr-install-linux-amd64 && sudo ./chr-install-linux-amd64
```

The wizard will:

1. Inspect the host, root-disk ancestry, boot method, active routes, addresses, resolver state, network configuration files, leases, and DHCP availability.
2. Let you review and edit the proposed IPv4, IPv6, DNS, gateway, MAC, and MTU plan.
3. Download the latest RouterOS 7 long-term CHR image and its official SHA-256 checksum from MikroTik.
4. Parse the image partition table and inject only an idempotent `/rw/autorun.scr` network plan.
5. Require explicit recovery, unverified-network, untested-release, disk-erasure, and reboot acknowledgements when applicable.
6. Write from rescue Linux or a RAM-backed pre-root environment, then verify every written image byte before booting CHR.

There is no unattended mode and no reboot countdown.

During the default interactive run, if the only blockers are missing `mkinitramfs` and/or `lsinitramfs` on a supported Debian or Ubuntu host, the installer shows the blocked report and offers to install `initramfs-tools-core` through APT. It refreshes package metadata and verifies both commands before rerunning preflight. Package installation requires explicit confirmation; `--preflight` and `--version` never install packages.

## Safety model

- The target disk is derived from the filesystem backing `/`, then its ancestry and fingerprint are rechecked after review.
- Preflight and the RAM writer use the same sysfs collector for size, major/minor, driver, serial, and WWN; `lsblk` is used only to resolve topology, path, and transport.
- SCSI identity prefers logical-unit VPD page `0x83` NAA, EUI-64, T10-vendor, and vendor-specific descriptors, in that order. Page `0x80` and direct sysfs serials are fallbacks only when page `0x83` has no usable logical-unit identity.
- If no stable identity is observable across reboot, installation is allowed only while exactly one physical disk is visible. The writer then requires kernel name, major/minor, size, and driver to remain identical and emits a `disk-identity` warning. A multi-disk host is blocked.
- A stable serial or WWN recorded before reboot may never degrade to the single-disk fallback; disappearance or change halts before writing.
- A normal running root disk is never overwritten from normal userspace; `mkinitramfs` rebuilds a RAM writer with the current kernel's full driver set.
- The built initramfs is inventoried before reboot; its compressed and unpacked sizes plus a runtime reserve must fit in installed RAM.
- GRUB staging requires a plain ext2/3/4 boot filesystem, installs dedicated `next_entry` handling, and verifies the armed one-shot entry before rebooting.
- The CHR filesystem offset is read from its MBR; it is not hard-coded.
- UEFI preparation requires the exact validated hybrid MBR/GPT geometry, matching primary and backup GPT CRCs, a bounded x86-64 EFI loader, and a bounded boot map. The generated FAT16 tables and copied files are read back before the image is authorized.
- DHCP probing sends only a DHCPDISCOVER packet and never installs an address or route on Linux.
- A non-default DHCP route needs no explicit RouterOS translation only when every observed forwarding attribute except its destination matches the selected DHCP default route. Different gateways, interfaces, protocols, metrics, preferred sources, scopes, flags, or other route attributes remain blockers.
- The first-boot script identifies the RouterOS uplink using the existing virtual NIC MAC.
- An unknown RouterOS version must pass structural checks and requires a typed acknowledgement. An incompatible layout is blocked.
- A failed read-back checksum halts the writer instead of rebooting.

## Recovery after interrupted staging

Returned staging errors automatically unload a pending kexec image, clear an armed GRUB entry, remove installer-owned files, and regenerate `grub.cfg`. A power loss or forced process kill can bypass that cleanup. Before a later normal reboot, remove only these installer-owned artifacts:

```bash
sudo grub-editenv /boot/grub/grubenv unset next_entry
sudo rm -f /etc/grub.d/42_chr_install /boot/chr-install/initrd.img /var/lib/chr-install/initrd.img
sudo rmdir /boot/chr-install /var/lib/chr-install 2>/dev/null || true
sudo update-grub
```

Timestamp refusals print the manifest time, writer clock, and bounded RTC adjustment. Restage from normal Linux after correcting a badly skewed RTC; do not boot an old writer entry manually.

## Building

```bash
go test ./...
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o dist/chr-install-linux-amd64 ./cmd/chr-install
```

The full CHR/QEMU test is opt-in locally because it downloads MikroTik's official image and starts a virtual machine. GitHub also runs it automatically every Monday against the latest RouterOS long-term release, so upstream image changes can require compatibility validation even when this repository has not changed:

```bash
sudo env CHR_QEMU_INTEGRATION=1 go test -tags=integration -run TestQEMUBoot ./internal/integration -v
```

The release workflow also runs privileged network-namespace scenarios and boots the pre-root writer against disposable virtio, SCSI, and NVMe disks. The SCSI workflow first performs a read-only probe boot, builds its manifest from the observed sysfs fingerprint, and then requires the writer boot to reproduce that identity. Its CHR matrix logs in through the untouched serial console and verifies the MAC-selected interface, address binding, routes, DNS, MTU, DHCP cleanup, and gateway reachability. Those tests are required before assets are published; a post-release smoke job then exercises both the canonical and legacy-redirect raw `installer.sh` URLs and their checksum bootstrap.

## Project history

This is an in-place rewrite of `parhamfa/install-mikrotik-chr-script`. The repository history and contributor attribution remain intact. The last DHCP-only shell implementation is preserved under the [`legacy-shell-final`](https://github.com/parhamfa/chr-install/tree/legacy-shell-final) tag.

The same repository was renamed to `parhamfa/chr-install` after the [`v1.0.0`](https://github.com/parhamfa/chr-install/releases/tag/v1.0.0) assets, full QEMU matrix, and legacy bootstrap URL passed their release gates. GitHub redirects the retired web, Git, raw-file, and release URLs. The old repository name must never be reused because that would break those redirects.

## Security

Report vulnerabilities privately according to [SECURITY.md](SECURITY.md). Do not disclose a disk-writing or network-preservation vulnerability in a public issue before it can be assessed.

## License

The v1 implementation is available under the [MIT License](LICENSE).
