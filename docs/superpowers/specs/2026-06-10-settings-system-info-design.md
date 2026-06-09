# Settings System Info Design

## Goal

Show read-only system information on the Settings page so an administrator can quickly see the host operating system, OS/kernel version, CPU architecture, and a few runtime details.

## API

Add `GET /api/settings/system-info`.

Response:

```json
{
  "name": "Linux",
  "version": "6.8.0-124-generic",
  "architecture": "amd64",
  "go_version": "go1.25.0",
  "cpu_count": 8,
  "hostname": "tg-search-host"
}
```

`version` is the Linux kernel release when running on Linux. On other platforms it may be empty if the standard runtime cannot provide an OS version without platform-specific dependencies.

## UI

Add a `系统` panel to `web/src/views/SettingsView.vue` and load it on mount with the other Settings data. Display `名称`, `版本`, `架构`, `主机名`, `CPU`, and `Go 版本`.

## Testing

Add a Go API test for the new endpoint and a Vue test that confirms the Settings page requests and renders the system information.
