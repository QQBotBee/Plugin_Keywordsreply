# GDI+ Modern Settings UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the standard-control settings window with the approved A-style modern flat GDI+ interface while retaining native text editing, all rule operations, accessibility, DPI behavior, and lifecycle guarantees.

**Architecture:** Put DPI-scaled geometry, hit testing, focus order, and reply-type constraints in a platform-neutral visual model. Keep the Windows lifecycle and controller integration in `settings.go`, and isolate GDI+/double-buffer rendering in `settings_gdiplus_windows.go`; only the keyword and reply-content `EDIT` controls remain native overlays.

**Tech Stack:** Go 1.22, Win32/386, GDI+, GDI double buffering, native Win32 `EDIT`, existing `SettingsController` and `RuleDraft`.

## Global Constraints

- Preserve `showSettingsWindow`, `closeSettingsWindow`, topmost single-instance behavior, host watcher, close waiting, and lazy `RuleStore` initialization.
- Preserve exact settings behavior: add, save, delete, reorder, validation, immediate persistence, media/channel disabling, and Chinese messages.
- Use the palette, 980×640 base client size, 4/8px spacing, 4～6px radius, typography, and high-contrast fallback from `docs/superpowers/specs/2026-08-08-gdiplus-settings-ui-design.md`.
- GDI+ draws all non-text-entry controls; only the keyword and reply-content fields remain native `EDIT` controls.
- Keep keyboard navigation, Chinese IME, clipboard shortcuts, visible focus, and 100%/125%/150% DPI layouts.
- Do not add external dependencies or modify the Bee ABI, JSON Lines IPC, `build.bat`, or `other/BeePlugin.def`.

---

### Task 1: Pure Visual Layout and Interaction Model

**Files:**
- Create: `settings_visual.go`
- Create: `settings_visual_test.go`

**Interfaces:**
- Consumes: `RuleDraft`, `ReplyType`, and the existing settings base size.
- Produces: `visualRect`, `visualTarget`, `settingsVisualLayout`, `newSettingsVisualLayout(dpi int) settingsVisualLayout`, `settingsVisualLayout.HitTest(x, y int, ruleCount int) visualTarget`, `settingsVisualFocusOrder() []visualTargetKind`, and `normalizeVisualDraft(RuleDraft) RuleDraft`.

- [ ] **Step 1: Write failing layout and hit-test tests**

```go
func TestSettingsVisualLayoutScalesWithoutOverlap(t *testing.T) {
    for _, dpi := range []int{96, 120, 144} {
        layout := newSettingsVisualLayout(dpi)
        if layout.Client.W != scaleDPI(980, dpi) || layout.Client.H != scaleDPI(640, dpi) {
            t.Fatalf("dpi=%d client=%+v", dpi, layout.Client)
        }
        for _, pair := range [][2]visualRect{
            {layout.Sidebar, layout.Editor},
            {layout.RuleList, layout.ListToolbar},
            {layout.ContentFrame, layout.BottomBar},
        } {
            if pair[0].Overlaps(pair[1]) { t.Fatalf("dpi=%d overlap=%+v", dpi, pair) }
        }
    }
}

func TestSettingsVisualHitTestFindsSaveAndRule(t *testing.T) {
    layout := newSettingsVisualLayout(96)
    if got := layout.HitTest(layout.Save.X+2, layout.Save.Y+2, 3); got.Kind != visualTargetSave {
        t.Fatalf("save target=%+v", got)
    }
    if got := layout.HitTest(layout.RuleList.X+4, layout.RuleList.Y+layout.RuleItemHeight+4, 3); got.Kind != visualTargetRule || got.Index != 1 {
        t.Fatalf("rule target=%+v", got)
    }
}
```

- [ ] **Step 2: Run tests to verify RED**

Run: `go test ./... -run 'TestSettingsVisual' -v`

Expected: FAIL because the visual model symbols do not exist.

- [ ] **Step 3: Implement scaled geometry and state normalization**

Use base-pixel rectangles for the 980×640 design and scale every coordinate through `scaleDPI`. `visualRect.Contains` uses left/top inclusive and right/bottom exclusive bounds; `Overlaps` returns false for touching edges. `HitTest` checks primary actions before containers and maps list-row Y positions to a zero-based rule index. `normalizeVisualDraft` clears `AreaChannel` and `AreaChannelPrivate` for audio, video, and file reply types.

- [ ] **Step 4: Verify GREEN**

Run: `gofmt -w settings_visual.go settings_visual_test.go && go test ./... -run 'TestSettingsVisual' -v && go test ./...`

Expected: PASS.

- [ ] **Step 5: Commit**

```powershell
git add settings_visual.go settings_visual_test.go
git commit -m "feat: add settings visual model"
```

### Task 2: GDI+ Double-Buffered Renderer

**Files:**
- Create: `settings_gdiplus_windows.go`
- Modify: `settings.go`

**Interfaces:**
- Consumes: `settingsVisualLayout`, `settingsUI`, controller rule snapshots, hover/press/focus targets, status text, and high-contrast state.
- Produces: `newSettingsRenderer(hwnd uintptr) (*settingsRenderer, error)`, `settingsRenderer.Paint(hwnd uintptr, ui *settingsUI)`, `settingsRenderer.Dispose()`, and `settingsRenderer.ApplyEditTheme(hwnd uintptr, hdc uintptr) uintptr`.

- [ ] **Step 1: Add a failing renderer contract test**

Add a platform-neutral palette test to `settings_visual_test.go`:

```go
func TestSettingsVisualPaletteUsesApprovedATheme(t *testing.T) {
    palette := normalSettingsPalette()
    if palette.Background != 0xFFF4F6F8 || palette.Primary != 0xFF0F766E || palette.Surface != 0xFFFFFFFF {
        t.Fatalf("palette=%+v", palette)
    }
}
```

- [ ] **Step 2: Run the focused test to verify RED**

Run: `go test ./... -run TestSettingsVisualPalette -v`

Expected: FAIL because `normalSettingsPalette` does not exist.

- [ ] **Step 3: Implement palette and renderer resources**

Define semantic ARGB palette fields in `settings_visual.go`. In `settings_gdiplus_windows.go`, load GDI+ and GDI procedures, start one GDI+ token per renderer, create `Microsoft YaHei UI` fonts, and dispose every token/font/brush/bitmap/DC on the owning window thread. Detect `SPI_GETHIGHCONTRAST`; map high-contrast colors from `GetSysColor`.

- [ ] **Step 4: Implement double-buffer painting**

For each `WM_PAINT`, create a compatible memory DC/bitmap sized to the client, create a GDI+ graphics object from the memory DC, draw the background, title, sidebar, list rows, fields, segmented controls, check options, tool buttons, bottom status, and save button, then `BitBlt` once to the paint DC. Return 1 from `WM_ERASEBKGND` and never draw directly from mouse/keyboard handlers.

- [ ] **Step 5: Integrate renderer lifecycle**

Create the renderer during `WM_CREATE`, call `Paint` during `WM_PAINT`, return the renderer's edit brush during `WM_CTLCOLOREDIT`, and dispose it during `WM_DESTROY`. A renderer creation failure must show a Chinese error, destroy the window, and allow `closeSettingsWindow` to complete.

- [ ] **Step 6: Verify compile and static checks**

Run: `gofmt -w settings_visual.go settings_gdiplus_windows.go settings.go && go test ./... && go vet ./...`

Expected: PASS with zero vet findings.

- [ ] **Step 7: Commit**

```powershell
git add settings_visual.go settings_visual_test.go settings_gdiplus_windows.go settings.go
git commit -m "feat: render modern settings with GDI+"
```

### Task 3: Hybrid Native Input and Custom Interaction

**Files:**
- Modify: `settings.go`
- Modify: `settings_visual.go`
- Modify: `settings_visual_test.go`

**Interfaces:**
- Consumes: Task 1 hit targets/focus order and Task 2 renderer.
- Produces: mouse hover/press/click handling, custom keyboard focus, native edit placement, tooltips, list selection, segmented selectors, check options, and controller actions.

- [ ] **Step 1: Write failing interaction-state tests**

```go
func TestNormalizeVisualDraftDisablesChannelMediaAreas(t *testing.T) {
    draft := RuleDraft{ReplyType: ReplyAudio, AreaFriend: true, AreaChannel: true, AreaChannelPrivate: true}
    got := normalizeVisualDraft(draft)
    if !got.AreaFriend || got.AreaChannel || got.AreaChannelPrivate { t.Fatalf("draft=%+v", got) }
}

func TestSettingsVisualFocusOrderIncludesNativeAndCustomControls(t *testing.T) {
    got := settingsVisualFocusOrder()
    for _, want := range []visualTargetKind{visualTargetRuleList, visualTargetKeyword, visualTargetMatchExact, visualTargetReplyText, visualTargetContent, visualTargetSave} {
        if !slices.Contains(got, want) { t.Fatalf("missing focus target %v", want) }
    }
}
```

- [ ] **Step 2: Run tests to verify RED**

Run: `go test ./... -run 'TestNormalizeVisualDraft|TestSettingsVisualFocusOrder' -v`

Expected: FAIL until focus order and state normalization are complete.

- [ ] **Step 3: Reduce native children to two EDIT controls**

Remove the native listbox, combos, checkboxes, buttons, and status controls. Create only `idKeyword` and `idReplyContent` as borderless `EDIT` children, move them to the rectangles from `settingsVisualLayout`, set the system font, and keep all existing text read/write and IME behavior.

- [ ] **Step 4: Add pointer and tooltip behavior**

Handle `WM_MOUSEMOVE`, `WM_MOUSELEAVE`, `WM_LBUTTONDOWN`, and `WM_LBUTTONUP`. Store hover and pressed targets in `settingsUI`, capture/release the mouse, invalidate on state changes, and invoke an action only when mouse-up hits the pressed target. Register native tooltip text for add/delete/up/down hit rectangles.

- [ ] **Step 5: Add keyboard and focus behavior**

Handle `WM_KEYDOWN`, `WM_SETFOCUS`, and `WM_KILLFOCUS` for custom targets. Tab/Shift+Tab traverses `settingsVisualFocusOrder`; arrow keys change rules/segments; Space toggles check options; Enter activates save or loads the focused rule. Move actual focus into the native edit HWNDs for keyword/content targets and draw focus rings for custom targets.

- [ ] **Step 6: Route actions through SettingsController**

Map custom targets to the current `RuleDraft`, call `Add`, `Update`, `Delete`, or `Move`, preserve text on errors, refresh the selected rule on success, and update the GDI+ status line. Selecting audio/video/file runs `normalizeVisualDraft` before repainting.

- [ ] **Step 7: Verify behavior and regressions**

Run: `gofmt -w settings.go settings_visual.go settings_visual_test.go && go test ./... && go vet ./...`

Expected: PASS.

- [ ] **Step 8: Commit**

```powershell
git add settings.go settings_visual.go settings_visual_test.go
git commit -m "feat: add custom settings interactions"
```

### Task 4: Documentation, Build, and Native Acceptance

**Files:**
- Modify: `docs/设置窗口开发规范.md`
- Modify: `docs/superpowers/keyword-reply-progress.md`

**Interfaces:**
- Documents: GDI+ renderer ownership, native edit boundary, visual states, accessibility, DPI checks, and Bee acceptance.

- [ ] **Step 1: Update settings documentation**

Replace the standard-control description with the approved hybrid renderer, palette, native `EDIT` boundary, double buffering, custom focus behavior, high-contrast fallback, and resource disposal rules.

- [ ] **Step 2: Run complete automated verification**

Run PowerShell enumeration for all Go files, then `gofmt -w`, `go test ./...`, and `go vet ./...`.

Expected: zero test failures and zero vet findings.

- [ ] **Step 3: Build and inspect the distributable**

Run: `cmd /c build.bat KeywordReply.dll`

Inspect the PE header and require machine `0x014C`, optional magic `0x010B`, and the DLL characteristic. Confirm root `worker_runtime.go` and `temp/` are absent.

- [ ] **Step 4: Perform real Bee visual acceptance**

Verify nonblank GDI+ drawing, no flicker, hover/pressed/focus/disabled states, Chinese IME and clipboard, Tab/arrow keys, all rule operations, 100%/125%/150% DPI, high contrast, single instance, topmost, disable close, and unload stability.

- [ ] **Step 5: Commit documentation**

```powershell
git add docs/设置窗口开发规范.md docs/superpowers/keyword-reply-progress.md
git commit -m "docs: document GDI+ settings UI"
```

- [ ] **Step 6: Final branch review**

Run: `git status --short --branch && git log --oneline --decorate -10`

Expected: a clean feature branch with separate visual-model, renderer, interaction, and documentation commits.
