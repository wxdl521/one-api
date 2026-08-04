const { contextBridge } = require('electron');

contextBridge.exposeInMainWorld('electron', {
  isElectron: true,
  version: process.versions.electron,
  platform: process.platform,
  versions: process.versions,
  // 远程模式下无本地数据目录；保留字段兼容 web 端读取（database-step.tsx）
  dataDir: undefined
});
