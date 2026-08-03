# Remote Desktop Installer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Windows NSIS installer that opens the deployed The One service without packaging or starting a local Go backend.

**Architecture:** Replace the current Electron production startup flow with a thin remote-client shell that loads `https://the-one.bolierxiang.cn/`. Keep native window and tray behavior, but remove server process management and backend binary resources. The hosted UI and its API share one HTTPS origin, so no client-side API base URL or CORS change is required.

**Tech Stack:** Electron 39, electron-builder 26, NSIS, Node.js.

---

### Task 1: Convert Electron runtime to a remote client

**Files:**
- Modify: `electron/main.js`
- Test: `electron/main.js` syntax check

- [ ] **Step 1: Verify the current startup behavior is local-server based**

Run: `rg -n "spawn\(|startServer|loadURL" electron/main.js`

Expected: `startServer` and a `127.0.0.1` `loadURL` are present.

- [ ] **Step 2: Replace the main-process startup implementation**

Implement a focused Electron main process with the following constants and behaviors:

```js
const REMOTE_APP_URL = 'https://the-one.bolierxiang.cn/'

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1080,
    height: 720,
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      nodeIntegration: false,
      contextIsolation: true,
    },
    title: 'The One',
    icon: path.join(__dirname, 'icon.png'),
  })
  mainWindow.loadURL(REMOTE_APP_URL)
}
```

Do not import `child_process`, do not spawn a Go executable, and do not create or configure a local data directory. Preserve tray show/hide and explicit Quit behavior.

- [ ] **Step 3: Validate JavaScript syntax and remote URL**

Run: `node --check electron/main.js; rg -n "REMOTE_APP_URL|spawn\(|127\.0\.0\.1" electron/main.js`

Expected: syntax succeeds; the configured HTTPS URL is found; no `spawn` or loopback load URL remains.

- [ ] **Step 4: Commit the runtime conversion**

```bash
git add electron/main.js
git commit -m "feat: load hosted app in desktop client"
```

### Task 2: Make Windows packaging frontend-client-only

**Files:**
- Modify: `electron/package.json`
- Test: `electron/package.json` validation

- [ ] **Step 1: Define a Windows installer-only build command**

Add the following script:

```json
"build:remote:win": "electron-builder --win nsis"
```

Set the Windows target to `nsis` only. Remove the `../the-one.exe` entry from `win.extraResources`; retain license resources.

- [ ] **Step 2: Validate the manifest contract**

Run:

```bash
node -e "const p=require('./electron/package.json'); if (p.build.win.target[0] !== 'nsis' || p.build.win.extraResources.some(r => r.from === '../the-one.exe')) process.exit(1)"
```

Expected: exit code 0.

- [ ] **Step 3: Commit the packaging configuration**

```bash
git add electron/package.json
git commit -m "build: package remote Windows client"
```

### Task 3: Produce and verify the installer

**Files:**
- Output: `electron/dist/*.exe` (ignored build artifact)

- [ ] **Step 1: Install locked Electron dependencies**

Run: `npm ci`

Working directory: `electron/`

Expected: dependencies install without modifying `package-lock.json`.

- [ ] **Step 2: Build the NSIS installer**

Run: `npm run build:remote:win`

Working directory: `electron/`

Expected: an NSIS installer is created in `electron/dist/`.

- [ ] **Step 3: Verify the output does not bundle the Go backend**

Run:

```powershell
Get-ChildItem -Path electron\dist -Filter '*.exe' -File
Test-Path electron\dist\win-unpacked\resources\bin\the-one.exe
```

Expected: an installer `.exe` is listed and the second command prints `False`.

- [ ] **Step 4: Report the installer path and connection URL**

Report the generated installer filename and confirm it opens `https://the-one.bolierxiang.cn/`.
