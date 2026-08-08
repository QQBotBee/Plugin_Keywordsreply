# Repository Guidelines

## Project Structure & Module Organization

This repository is a Go 1.22 SDK/template for Bee plugins on 32-bit Windows. Keep plugin-specific behavior in `plugin_main.go`; customize the native settings UI in `settings.go`. `bee_sdk.go` contains the public Bee API, event constants, and IPC client. Low-level bridge code lives in `other/`: `bee_bridge.c` implements the C DLL shell, `worker_runtime.go` dispatches events to the Go worker, `BeePlugin.def` defines exports, and `buildmeta/` generates build metadata. Reference material is under `docs/`. Generated files belong in `build/` or `temp/` and must not be committed.

## Build, Test, and Development Commands

- `go test ./...` compiles all Go packages and runs any tests.
- `go vet ./...` checks Go code for common correctness problems.
- `gofmt -w plugin_main.go settings.go` formats edited Go files; include other changed `.go` files as needed.
- `build.bat` builds the Windows/386 worker, embeds it, and links `build\<PluginName>.dll`. Go and Zig must be on `PATH`.
- `build.bat MyPlugin.dll` overrides the output DLL name.

Run the batch build from Windows. Validate loading, callbacks, settings UI, disable/unload behavior, and Worker cleanup in a real Bee installation.

## Coding Style & Naming Conventions

Follow standard Go formatting and naming: tabs via `gofmt`, PascalCase for exported identifiers, camelCase for internal identifiers, and short lowercase package names. Keep callbacks named consistently with existing handlers such as `onGroupMessage`. Preserve the Windows/386 stdcall ABI and the JSON Lines IPC contract. Bee-facing text uses GBK/CP936 while Go code uses UTF-8. Do not convert `build.bat` or `other/BeePlugin.def`; `.gitattributes` intentionally treats both as binary.

## Testing Guidelines

There is currently no automated test suite. Add Go tests beside the code under test using `*_test.go` names and `TestXxx` functions; prefer table-driven cases for protocol encoding and helpers. Always run `go test ./...` and `go vet ./...`. Changes to lifecycle, message handling, IPC, or the settings window also require manual Bee verification.

## Commit & Pull Request Guidelines

No commit-message convention can be inferred because the repository has no commits yet. Use concise, imperative subjects (for example, `Fix worker shutdown timeout`) and keep each commit focused. Pull requests should explain behavior and architecture impact, list commands and Bee scenarios tested, link relevant issues, and include screenshots for settings-window changes. Never include DLL, EXE, RES, LIB, PDB, `build/`, or `temp/` artifacts.
