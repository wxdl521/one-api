# WeChat Mini Program (phase one)

This is a standalone Taro 4 workspace for the WeChat Mini Program. It is not part of the browser `web/` application and does not share its React 19 dependencies. The phase-one shell makes no network requests. Future requests must target `/api/miniapp/v1` only.

## Install and verify

Run these commands from this directory:

```powershell
bun install
bun run dev:weapp
bun run build:weapp
bun run typecheck
bun run test
bun run lint
```

`bun run dev:weapp` watches source files and writes the development build to `dist/`. `bun run build:weapp` writes a one-time WeChat Mini Program build to the same directory.

The pinned `@tarojs/webpack5-runner` development dependency is required by Taro's `webpack5` compiler setting to produce the WeChat build. `babel.config.js` enables Taro's React and TypeScript transform; it is required for Taro to compile the `.tsx` source files.

## Import into WeChat Developer Tools

1. Run `bun run dev:weapp`.
2. In WeChat Developer Tools, choose **Import Project**.
3. Select this `miniapp/` directory, not the browser `web/` directory. Its `project.config.json` points Developer Tools at `dist/`.
4. Use the tourist project for local shell work, or select the approved AppID in Developer Tools or CI for real integration work.

The tracked project configuration deliberately uses `touristappid`. A real AppID is selected in Developer Tools or injected by CI; it must not be committed to client source.

## Development base URL

`config/index.ts` reads `TARO_APP_API_BASE_URL` and `TARO_APP_MINIAPP_BINDING_ORIGIN` while compiling. They become the public `__MINIAPP_API_BASE_URL__` and `__MINIAPP_BINDING_ORIGIN__` constants. No URL is committed. Runtime API calls target only `/api/miniapp/v1`, require an HTTPS base URL, and fail closed when the compiled value is missing or invalid. Browser account binding additionally accepts only the configured HTTPS origin's exact `/miniapp-bind` page.

For PowerShell, switch a local development build with:

```powershell
$env:TARO_APP_API_BASE_URL = 'https://<development-gateway>'
$env:TARO_APP_MINIAPP_BINDING_ORIGIN = 'https://<development-console>'
bun run dev:weapp
```

For a different environment, replace the value and restart the command. To verify the fail-closed configuration state locally, run:

```powershell
Remove-Item Env:TARO_APP_API_BASE_URL
Remove-Item Env:TARO_APP_MINIAPP_BINDING_ORIGIN
bun run dev:weapp
```

## HTTPS and real-device testing

Before adding or exercising API calls on a real device, configure the selected AppID's **request legal domain** in the WeChat Mini Program administration console. The domain must use publicly trusted HTTPS and satisfy WeChat's current domain, certificate, and TLS requirements. Developer Tools can relax domain checks only for local development; that does not make a domain valid for a real device.

For a device test, choose the approved real AppID in Developer Tools or CI, build the Mini Program, generate a preview, and open it from WeChat on a physical device. Verify the device can reach the HTTPS gateway over its real network and that the configured request domain is accepted before testing authentication or any future `/api/miniapp/v1` endpoint.
