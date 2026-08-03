const { app, BrowserWindow, Menu, Tray } = require('electron')
const path = require('path')

const REMOTE_APP_URL = 'https://the-one.bolierxiang.cn/'

let mainWindow = null
let tray = null

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

app.whenReady().then(() => {
  createTray()
  createWindow()
})

app.on('activate', () => {
  if (BrowserWindow.getAllWindows().length === 0) {
    createWindow()
  }
})

app.on('window-all-closed', () => {
  // Keep the app available from the system tray until the user explicitly quits.
})
