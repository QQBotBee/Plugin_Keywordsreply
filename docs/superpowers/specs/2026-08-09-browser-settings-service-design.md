# Browser Settings Service Design

## Goal

Replace the current native rule editor with a lightweight native control window that lets the user manually start and stop a local HTTP settings service. The browser page served by that local service becomes the rule editing UI.

## Confirmed Requirements

- The HTTP service must never auto-start.
- Clicking plugin settings opens only the native control window.
- The service starts only when the user clicks an enable button.
- The default port is `6655`.
- The user can edit the port before starting the service.
- The control window includes a port availability check button.
- Port checks are advisory; service startup must still bind the port again.
- The service listens only on `127.0.0.1`.
- If the service starts successfully, the plugin opens the default browser.
- If the plugin is disabled or unloaded while the service is running, the service is stopped.
- Runtime state is not persisted. A plugin restart always leaves the service stopped.
- The last configured port may be persisted for convenience.
- No web framework, middleware, database, WebSocket, Node.js runtime, or external frontend runtime is required.

## Architecture

The existing Go Worker remains the only long-running plugin process. The native settings entry point creates a small Win32 control window. The control window owns no rule-editing behavior; it only manages the HTTP service port and lifecycle.

The HTTP service is implemented with Go standard library `net/http`, `net`, and `embed`. Static HTML, CSS, and JavaScript are compiled into the Worker. API handlers convert browser requests into existing `RuleStore` operations, so all validation and atomic JSON persistence remain centralized in Go.

## Components

- `settings.go`: native Win32 service control window.
- `settings_web.go`: local HTTP service lifecycle, port validation, browser opening, static file serving, and JSON API.
- `web/settings/`: embedded browser UI assets.
- `docs/superpowers/plans/2026-08-09-browser-settings-service.md`: implementation plan.

## Data Flow

1. User clicks plugin settings.
2. Worker opens native control window.
3. User enters or accepts port `6655`.
4. User optionally checks whether the port can bind.
5. User clicks enable.
6. Worker binds `127.0.0.1:<port>` and starts `http.Server`.
7. Worker opens the default browser at `http://127.0.0.1:<port>/?token=<random>`.
8. Browser calls JSON APIs to read or replace keyword rules.
9. Go validates and persists rules through `RuleStore.Replace`.
10. User stops the service, or plugin disable/unload stops it.

## Security

- Bind only to loopback.
- Require an unguessable per-process token on browser page and API requests.
- Reject unsupported HTTP methods and malformed JSON.
- Do not expose arbitrary file access.

## Error Handling

- Invalid ports show a status message and do not start the service.
- Occupied ports are reported from both the check button and the start action.
- Browser launch failure is reported in the control window and Bee log when available.
- API validation errors return HTTP `400` with a JSON error message.
- Service shutdown uses `http.Server.Shutdown` with a short timeout, then clears runtime state.

## Verification

- Run `go test ./...` to compile all Go packages.
- Run `go vet ./...` to catch common correctness issues.
- Run `build.bat KeywordReply.dll` on Windows after code changes.
- Do not leave temporary `*_test.go` files in the repository after implementation checks.
