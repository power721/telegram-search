# Settings Version Check Design

## Goal

The settings page shows the currently running tg-search version and lets an administrator check GitHub Releases for the latest published version.

## User Experience

The settings page adds a compact version panel. On load, the panel displays the current backend version. The panel includes a "检查更新" button. When the administrator clicks it, the UI calls the backend, shows a loading state, then displays one of these outcomes:

- Current version is up to date.
- A newer GitHub Release is available, with a link to the release page.
- The update check failed, with a short error message.

The current version should still be visible if the GitHub check fails. If the binary was built without an injected version, the backend reports `dev`.

## Backend Design

Add `GET /api/settings/version` under the existing settings API surface. The endpoint returns:

```json
{
  "current_version": "v1.2.3",
  "latest_version": "v1.2.4",
  "latest_url": "https://github.com/power721/tg-search/releases/tag/v1.2.4",
  "update_available": true
}
```

The current version comes from `internal/build.Version`.

The backend owns the GitHub API call to `https://api.github.com/repos/power721/tg-search/releases/latest`. Keeping this server-side avoids browser CORS issues and keeps the frontend independent from GitHub response details.

Version comparison strips a leading `v` and compares semantic numeric parts. If either version is `dev`, empty, or not parseable, the response should not report `update_available: true`; it can still return the latest release version and URL.

GitHub request failures return a non-2xx API error from `/api/settings/version`. The frontend handles that as a failed check while preserving the already displayed current version when possible.

## Frontend Design

Add a `VersionInfoResponse` type to `web/src/api/types.ts`.

`SettingsView.vue` loads `/api/settings/version` on mount so the current version is shown without requiring a button click. The same request also populates latest-release fields when GitHub is reachable. The "检查更新" button re-runs the request and uses the same response.

The version panel follows the existing settings layout and uses Naive UI buttons like the API key panel. It should not introduce a new global store because this data is local to the settings page.

## Testing

Backend tests cover:

- `/api/settings/version` returns the injected current version.
- A newer GitHub Release sets `update_available` to `true`.
- Equal or older releases do not set `update_available`.
- `dev` current version does not claim an update is available.

Frontend tests cover:

- Settings page requests `/api/settings/version` on mount and displays the current version.
- Clicking "检查更新" requests the endpoint again and displays the latest version state.

## Scope

This change does not implement automatic download, installation, restart, changelog rendering, or release asset selection.
