const { app, BrowserWindow, Menu, Tray, session, shell } = require('electron')
const path = require('path')
const { pathToFileURL } = require('url')

const REMOTE_APP_URL = process.env.THE_ONE_REMOTE_URL || 'https://the-one.bolierxiang.cn/'
const ALLOWED_ORIGIN = new URL(REMOTE_APP_URL).origin
// Local pages (e.g. offline.html) live in __dirname; navigation to them is allowed.
const LOCAL_FILE_URL_PREFIX = pathToFileURL(path.join(__dirname, path.sep)).href

let mainWindow = null
let tray = null

function isAllowedOrigin(url) {
  try {
    return new URL(url).origin === ALLOWED_ORIGIN
  } catch {
    return false
  }
}

// Popups the app opens itself (about:blank / same-origin, e.g. the OAuth
// account-binding window it drives via window.open then navigates to a provider)
// open in-app; external http(s) links go to the system browser; other schemes drop.
function windowOpenHandler({ url }) {
  if (url === '' || url === 'about:blank' || isAllowedOrigin(url)) {
    return { action: 'allow' }
  }
  if (url.startsWith('http:') || url.startsWith('https:')) {
    shell.openExternal(url)
  }
  return { action: 'deny' }
}

// Navigation scheme policy shared by will-navigate and will-redirect.
// Desktop client is a logged-in remote client: allow all top-level http(s)
// navigation (user-driven links + OAuth provider redirects). Block non-http(s)
// schemes (javascript:, data:, custom, arbitrary file:) except bundled local pages.
function guardNavigation(event, url) {
  let target
  try {
    target = new URL(url)
  } catch {
    event.preventDefault()
    return
  }
  if (target.protocol === 'http:' || target.protocol === 'https:') {
    return
  }
  if (target.protocol === 'file:' && target.href.startsWith(LOCAL_FILE_URL_PREFIX)) {
    return
  }
  event.preventDefault()
}

function createWindow() {
  mainWindow = new BrowserWindow({
    width: 1080,
    height: 720,
    minWidth: 960,
    minHeight: 640,
    title: 'The One',
    icon: path.join(__dirname, 'icon.png'),
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      nodeIntegration: false,
      contextIsolation: true,
    },
  })

  mainWindow.webContents.on('did-fail-load', (event, errorCode, errorDescription, validatedURL, isMainFrame) => {
    // -3 = ERR_ABORTED (navigation superseded), not a real failure.
    if (!isMainFrame || errorCode === -3) {
      return
    }
    mainWindow.loadFile(path.join(__dirname, 'offline.html'), {
      query: { retry: REMOTE_APP_URL },
    })
  })

  mainWindow.loadURL(REMOTE_APP_URL)

  mainWindow.on('close', (event) => {
    if (!app.isQuitting) {
      event.preventDefault()
      mainWindow.hide()
    }
  })

  mainWindow.on('closed', () => {
    mainWindow = null
  })
}

function showWindow() {
  if (mainWindow === null) {
    createWindow()
    return
  }

  mainWindow.show()
  mainWindow.focus()
}

function createTray() {
  const iconName = process.platform === 'darwin'
    ? 'tray-iconTemplate.png'
    : 'tray-icon-windows.png'
  tray = new Tray(path.join(__dirname, iconName))

  tray.setToolTip('The One')
  tray.setContextMenu(Menu.buildFromTemplate([
    {
      label: 'Show The One',
      click: showWindow,
    },
    { type: 'separator' },
    {
      label: 'Quit',
      click: () => {
        app.isQuitting = true
        app.quit()
      },
    },
  ]))

  tray.on('click', () => {
    if (mainWindow?.isVisible()) {
      mainWindow.hide()
      return
    }
    showWindow()
  })
}

// Apply the navigation scheme policy and popup handler to every web contents,
// including child windows the app opens (e.g. the OAuth binding popup), so a
// server-side 302 or a child-window navigation cannot escape the guard.
app.on('web-contents-created', (event, contents) => {
  contents.on('will-navigate', guardNavigation)
  contents.on('will-redirect', guardNavigation)
  contents.setWindowOpenHandler(windowOpenHandler)
})

app.whenReady().then(() => {
  // Remote web app needs no device/notification permissions; deny all except fullscreen (video playback).
  session.defaultSession.setPermissionRequestHandler((webContents, permission, callback) => {
    callback(permission === 'fullscreen')
  })

  createTray()
  createWindow()
})

app.on('activate', () => {
  // Window hidden via close-to-tray still counts in getAllWindows(), so show it
  // (or recreate if it was destroyed) rather than gating on window count.
  showWindow()
})

app.on('window-all-closed', () => {
  // Keep the app available from the system tray until the user explicitly quits.
})
