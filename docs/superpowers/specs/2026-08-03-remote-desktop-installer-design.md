# Remote Desktop Installer Design

## Goal

Create a Windows NSIS installer for The One that acts as a desktop client for the deployed service at `https://the-one.bolierxiang.cn/`.

## Scope

- Package Electron only; do not include the Go backend, SQLite database, or locally built web assets.
- Open the configured HTTPS service in a native Electron window.
- Retain the existing application icon and system-tray behavior.
- Produce a selectable-directory Windows NSIS installer.

## Architecture

The Electron main process will load `https://the-one.bolierxiang.cn/` directly and will not spawn a local server. The service hosts both the web UI and API on one HTTPS origin, so the existing relative API requests and cookie-based authentication continue to work without CORS changes.

## Packaging

The Windows electron-builder configuration will include only the Electron main process, preload script, and icons. It will remove the backend executable from `extraResources` and build the NSIS target only. A dedicated build command will generate the installer under `electron/dist/`.

## Validation

- Verify the target URL responds successfully over HTTPS.
- Run the Windows electron-builder command and confirm the installer is produced.
- Inspect the packaged resources to confirm no Go executable is bundled.
