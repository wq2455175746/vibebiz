package steam

import (
	"path/filepath"
	"strconv"
)

func mainJS(entry string) string {
	return `const { app, BrowserWindow } = require('electron')
const path = require('path')

function createWindow() {
  const win = new BrowserWindow({
    width: 1280,
    height: 720,
    webPreferences: {
      preload: path.join(__dirname, 'preload.js'),
      contextIsolation: true,
      nodeIntegration: false
    }
  })
  win.loadFile(path.join(__dirname, '..', 'app', ` + strconv.Quote(filepath.ToSlash(entry)) + `))
  win.setMenuBarVisibility(false)
}

app.whenReady().then(createWindow)

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') app.quit()
})
`
}

const preloadJS = `const { contextBridge } = require('electron')

contextBridge.exposeInMainWorld('vibebiz', {
  platform: process.platform
})
`

const packageJSON = `{
  "name": "vibebiz-steam-game",
  "version": "1.0.0",
  "main": "electron/main.js",
  "scripts": {
    "start": "electron .",
    "dist": "electron-builder --win --x64"
  },
  "devDependencies": {
    "electron": "^33.0.0",
    "electron-builder": "^25.0.0"
  },
  "build": {
    "appId": "com.vibebiz.game",
    "productName": "VibebizGame",
    "directories": { "output": "dist" },
    "win": { "target": ["zip"] }
  }
}
`

const electronBuilderYML = `appId: com.vibebiz.game
productName: VibebizGame
directories:
  output: dist
files:
  - electron/**/*
  - app/**/*
  - package.json
win:
  target:
    - zip
`

const buildSteamSH = `#!/usr/bin/env sh
set -eu
npm install
npm run dist
`

const buildSteamPS1 = `$ErrorActionPreference = "Stop"
npm install
npm run dist
`

const appBuildVDF = `"appbuild"
{
  "appid" "YOUR_APP_ID"
  "desc" "Vibebiz game build"
  "buildoutput" "..\\steam-output"
  "contentroot" "..\\dist\\win-unpacked"
  "depots"
  {
    "YOUR_DEPOT_ID" "depot_build.vdf"
  }
}
`
