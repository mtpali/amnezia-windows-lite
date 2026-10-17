# Windows Lite

A compact, OLED-black, portable Windows client based on [amneziawg-windows-client](https://github.com/amnezia-vpn/amneziawg-windows-client), retaining the compatible protocol engine.

Download the ZIP for your architecture from [Releases](https://github.com/mtpali/amnezia-windows-lite/releases), extract it, and run `WindowsLite.exe`. Administrator elevation is required for Windows networking. No installer is needed. See [PORTABLE.txt](PORTABLE.txt) for storage and exit behavior.

- OLED black throughout the application, including the configuration editor and app dialogs.
- Compact 480 × 310 logical-pixel main window; resizable, with keyboard shortcuts.
- Block untunneled traffic is an explicit, default-off setting. Full-tunnel routes are preserved.
- The last successfully started configuration is restored instead of selecting the alphabetically first row.
- No About menu, promotional footer, upstream updater, or MSI installer.
- Encrypted configuration storage next to the EXE; manual-start services removed on Exit.
- Lazy log viewer and reduced, visibility-aware polling.

## Build

From Windows 10/11 x64 PowerShell:

```powershell
./build.ps1 -Arch amd64 -RunTests
# Optional ARM64 build:
./build.ps1 -Arch arm64
```

`build.bat` invokes the same script. The script downloads and verifies Go 1.25.14 and Wintun 0.14.1. Resource generation uses pinned `github.com/akavel/rsrc@v0.10.2`. No C compiler or Visual Studio is needed. ZIPs and SHA256 hashes are written to `dist/`. GitHub Actions builds x64 and ARM64 and publishes the downloads.

Only targeted, deterministic tests are run. Inherited upstream integration tests are not run indiscriminately because some require a live system, large downloads, or never terminate.

## Source

Initial client source: upstream commit `800416d673d83ed90ede02ca2ccd3683fe107d12`.
The local `third_party/awg-windows` module is based on `v3.1.20260814`. Its local changes implement explicit kill-switch policy, separate service names, and portable file writes. Cryptography and the network protocol are retained.

The `BlockUntunneledTraffic = on` interface setting is specific to this client. With it absent or off, importing or using a full-tunnel configuration does not restrict untunneled traffic. Explicit opt-in applies to an eligible single-peer full tunnel with routing enabled.

Original copyrights and licenses are preserved. See [COPYING](COPYING) and [THIRD-PARTY-NOTICES.txt](THIRD-PARTY-NOTICES.txt).
