# Keyword Reply Plugin Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a configurable Bee keyword-reply plugin with ordered exact/fuzzy matching, four trigger areas, text/Markdown/media replies, JSON persistence, and a native Windows rule editor.

**Architecture:** Keep rules, matching, reply rendering, and the settings controller in small testable Go units. Message callbacks bind the current `BeeAPI` target, find the first matching immutable rule snapshot, and dispatch through `MessageTarget`; the Win32 layer edits the same `RuleStore` through a controller and persists each accepted change atomically.

**Tech Stack:** Go 1.22, standard library, Win32 API through `syscall`, Bee JSON Lines IPC, Zig/C bridge, Go `testing` package.

## Global Constraints

- Build target remains Windows/386 with `CGO_ENABLED=0`; do not change the stdcall DLL ABI or JSON Lines IPC contract.
- `build.bat` and `other/BeePlugin.def` remain binary-tracked GBK/CP936 files and must not be reformatted.
- Trigger keywords are unique by exact stored string; rule order determines first-match priority.
- Exact matching trims message edges and compares the whole message; fuzzy matching trims edges and uses substring containment.
- Case-sensitive matching is the per-rule default; case folding is used only when disabled.
- Audio, video, and file replies are limited to QQ friends and groups.
- Channel-private Markdown degrades to ordinary text; all callbacks return `MessageContinue`.
- Configuration is UTF-8 JSON at `GetAppDataDir()/keyword_replies.json` and is replaced atomically.

---

## File Structure

- Create `ipc_message.go`: build-tag-independent IPC envelope shared by SDK tests and the generated Worker.
- Create `keyword_config.go`: enums, rule validation, immutable snapshots, JSON loading/saving.
- Create `replace_file_windows.go`: atomic Windows replacement using `MoveFileExW`.
- Create `keyword_match.go`: ordered area-aware matching.
- Create `keyword_reply.go`: image-marker parsing, media line normalization, reply dispatch.
- Create `keyword_runtime.go`: global store initialization and callback-to-target coordination.
- Create `settings_model.go`: testable rule-editor controller.
- Modify `settings.go`: standard Win32 controls and event handling.
- Modify `bee_sdk.go`: `MessageTarget` methods for Markdown, audio, video, and files.
- Modify `plugin_main.go`: metadata, startup loading, settings binding, and four message callbacks.
- Create matching `*_test.go` files beside each pure Go module.
- Modify `README.md`: plugin configuration, matching, reply syntax, storage, and validation guidance.
- Modify `docs/设置窗口开发规范.md`: replace the template-only window description with the rule-editor contract.

### Task 1: Restore a Testable Baseline

**Files:**
- Create: `ipc_message.go`
- Modify: `other/worker_runtime.go:18-29`

**Interfaces:**
- Produces: `type IPCMessage struct` available to normal tests and the materialized Worker.
- Preserves: the existing IPC JSON field names and `omitempty` behavior.

- [ ] **Step 1: Verify the existing compile failure**

Run: `go test ./...`

Expected: FAIL in `bee_sdk.go` with `undefined: IPCMessage`.

- [ ] **Step 2: Move the shared envelope into normal package code**

Create `ipc_message.go` with the exact existing structure:

```go
package main

type IPCMessage struct {
    Type       string   `json:"type"`
    ID         string   `json:"id,omitempty"`
    Event      string   `json:"event,omitempty"`
    ArgsB64    []string `json:"args_b64,omitempty"`
    CommandB64 string   `json:"command_b64,omitempty"`
    ValueB64   string   `json:"value_b64,omitempty"`
    Result     int      `json:"result,omitempty"`
    Error      string   `json:"error,omitempty"`
}
```

Remove the duplicate declaration from `other/worker_runtime.go`; its build-time copy will compile against `ipc_message.go`.

- [ ] **Step 3: Verify the package baseline**

Run: `gofmt -w ipc_message.go other/worker_runtime.go && go test ./... && go vet ./...`

Expected: PASS; packages may report `[no test files]`.

- [ ] **Step 4: Commit the infrastructure fix**

```powershell
git add ipc_message.go other/worker_runtime.go
git commit -m "fix: share IPC message type with tests"
```

### Task 2: Rule Model, Validation, and Atomic Persistence

**Files:**
- Create: `keyword_config.go`
- Create: `replace_file_windows.go`
- Create: `keyword_config_test.go`

**Interfaces:**
- Produces: `MatchMode`, `TriggerArea`, `ReplyType`, `KeywordRule`.
- Produces: `ValidateRules([]KeywordRule) error`.
- Produces: `NewRuleStore(path string) *RuleStore`, `(*RuleStore).Load() error`, `Replace([]KeywordRule) error`, and `Snapshot() []KeywordRule`.

- [ ] **Step 1: Write failing validation and persistence tests**

Add table-driven tests that assert unique keywords, required keyword/area/content, legal enum values, media-area restrictions, missing-file loading, JSON round-trip, snapshot copy isolation, and preservation of the previous file when replacement fails. Core cases:

```go
func TestValidateRulesRejectsDuplicateKeyword(t *testing.T) {
    rules := []KeywordRule{
        {Keyword: "hello", MatchMode: MatchExact, CaseSensitive: true, Areas: []TriggerArea{AreaFriend}, ReplyType: ReplyText, Contents: []string{"one"}},
        {Keyword: "hello", MatchMode: MatchFuzzy, CaseSensitive: true, Areas: []TriggerArea{AreaGroup}, ReplyType: ReplyText, Contents: []string{"two"}},
    }
    if err := ValidateRules(rules); err == nil {
        t.Fatal("expected duplicate keyword error")
    }
}

func TestRuleStoreRoundTrip(t *testing.T) {
    path := filepath.Join(t.TempDir(), "keyword_replies.json")
    store := NewRuleStore(path)
    want := []KeywordRule{{Keyword: "菜单", MatchMode: MatchExact, CaseSensitive: true, Areas: []TriggerArea{AreaGroup}, ReplyType: ReplyText, Contents: []string{"请选择"}}}
    if err := store.Replace(want); err != nil { t.Fatal(err) }
    loaded := NewRuleStore(path)
    if err := loaded.Load(); err != nil { t.Fatal(err) }
    if got := loaded.Snapshot(); !reflect.DeepEqual(got, want) {
        t.Fatalf("got %#v want %#v", got, want)
    }
}
```

- [ ] **Step 2: Run the new tests to verify RED**

Run: `go test ./... -run 'TestValidateRules|TestRuleStore' -v`

Expected: FAIL because `KeywordRule`, enum constants, and `RuleStore` do not exist.

- [ ] **Step 3: Implement the model and store**

Define stable string enums and the approved JSON structure:

```go
type KeywordRule struct {
    Keyword       string        `json:"keyword"`
    MatchMode     MatchMode     `json:"match_mode"`
    CaseSensitive bool          `json:"case_sensitive"`
    Areas         []TriggerArea `json:"areas"`
    ReplyType     ReplyType     `json:"reply_type"`
    Contents      []string      `json:"contents"`
}

type RuleStore struct {
    mu          sync.RWMutex
    path        string
    rules       []KeywordRule
    replaceFile func(string, string) error
}
```

`NewRuleStore` sets `replaceFile` to `replaceFileAtomically`; tests replace that field with a function returning `errors.New("replace failed")` to prove the old file and in-memory snapshot remain unchanged. `Replace` validates a deep copy, writes indented JSON to a same-directory temporary file, calls `Sync` and `Close`, then invokes `replaceFile(temp, path)` before swapping the in-memory slice. `Load` treats `os.ErrNotExist` as an empty rule set and validates decoded data before publishing it. `Snapshot` deep-copies both the rule slice and nested slices.

Implement `replaceFileAtomically` with `MoveFileExW` flags `MOVEFILE_REPLACE_EXISTING|MOVEFILE_WRITE_THROUGH`; return the Win32 error when the call returns zero.

- [ ] **Step 4: Verify GREEN and static checks**

Run: `gofmt -w keyword_config.go replace_file_windows.go keyword_config_test.go && go test ./... -run 'TestValidateRules|TestRuleStore' -v && go vet ./...`

Expected: PASS.

- [ ] **Step 5: Commit the rule store**

```powershell
git add keyword_config.go replace_file_windows.go keyword_config_test.go
git commit -m "feat: add keyword rule persistence"
```

### Task 3: Ordered Matching Engine

**Files:**
- Create: `keyword_match.go`
- Create: `keyword_match_test.go`

**Interfaces:**
- Consumes: `KeywordRule`, `TriggerArea`, `MatchExact`, and `MatchFuzzy` from Task 2.
- Produces: `MatchKeywordRule(rules []KeywordRule, area TriggerArea, message string) (KeywordRule, bool)`.

- [ ] **Step 1: Write failing table-driven matching tests**

Cover trimmed exact matching, fuzzy containment, case-sensitive default behavior, explicit case-insensitive matching, area exclusion, and first-in-list precedence:

```go
func TestMatchKeywordRuleUsesListOrder(t *testing.T) {
    rules := []KeywordRule{
        {Keyword: "你好", MatchMode: MatchFuzzy, CaseSensitive: true, Areas: []TriggerArea{AreaGroup}},
        {Keyword: "你好呀", MatchMode: MatchExact, CaseSensitive: true, Areas: []TriggerArea{AreaGroup}},
    }
    got, ok := MatchKeywordRule(rules, AreaGroup, " 你好呀 ")
    if !ok || got.Keyword != "你好" { t.Fatalf("got %#v, %v", got, ok) }
}
```

- [ ] **Step 2: Run the tests to verify RED**

Run: `go test ./... -run TestMatchKeywordRule -v`

Expected: FAIL with `undefined: MatchKeywordRule`.

- [ ] **Step 3: Implement minimal ordered matching**

Trim only the incoming message edges. For each rule in order, skip rules without the active area; compare directly when case-sensitive and compare `strings.ToLower` values otherwise. Return immediately on exact equality or fuzzy `strings.Contains`; return the zero rule and `false` after the loop.

- [ ] **Step 4: Verify GREEN**

Run: `gofmt -w keyword_match.go keyword_match_test.go && go test ./... -run TestMatchKeywordRule -v`

Expected: PASS.

- [ ] **Step 5: Commit the matching engine**

```powershell
git add keyword_match.go keyword_match_test.go
git commit -m "feat: add ordered keyword matching"
```

### Task 4: Reply Parsing and Target Dispatch

**Files:**
- Create: `keyword_reply.go`
- Create: `keyword_reply_test.go`
- Modify: `bee_sdk.go:1009-1050`

**Interfaces:**
- Consumes: rule and area enums from Task 2.
- Produces: `ParseTextReply(string) (ParsedTextReply, error)` and `MediaItems([]string) []string`.
- Produces: `SendKeywordReply(keywordReplyTarget, TriggerArea, KeywordRule) (ReplyOutcome, error)`.
- Produces on `MessageTarget`: `SendMarkdown(string)`, `SendAudio(string)`, `SendVideo(string)`, and `SendFile(string)`.

- [ ] **Step 1: Write failing parser and dispatch tests**

Use a recording fake that implements all six sends. Assert plain text, one `[图片=...]` marker, empty/multiple marker errors, blank media-line filtering, ordered media calls, early stop on error, Markdown mapping, and channel-private Markdown degradation:

```go
func TestSendKeywordReplyDegradesChannelPrivateMarkdown(t *testing.T) {
    target := &recordingReplyTarget{}
    rule := KeywordRule{ReplyType: ReplyMarkdown, Contents: []string{"**你好**"}}
    outcome, err := SendKeywordReply(target, AreaChannelPrivate, rule)
    if err != nil { t.Fatal(err) }
    if !outcome.Degraded || !reflect.DeepEqual(target.calls, []string{"text:**你好**"}) {
        t.Fatalf("outcome=%+v calls=%v", outcome, target.calls)
    }
}
```

- [ ] **Step 2: Run the tests to verify RED**

Run: `go test ./... -run 'TestParseTextReply|TestMediaItems|TestSendKeywordReply' -v`

Expected: FAIL because reply parsing and dispatch symbols do not exist.

- [ ] **Step 3: Implement parsing and SDK target methods**

Use a compiled regexp matching `\[图片=([^\]\r\n]*)\]`. Return an error for an empty address or more than one match; otherwise remove the marker and trim the remaining text. Normalize media by splitting every stored content string on CR/LF, trimming lines, and dropping empties.

Define:

```go
type keywordReplyTarget interface {
    SendText(string) (string, error)
    SendImage(string, string) (string, error)
    SendMarkdown(string) (string, error)
    SendAudio(string) (string, error)
    SendVideo(string) (string, error)
    SendFile(string) (string, error)
}

type ReplyOutcome struct {
    Sent     int
    Degraded bool
}
```

`SendKeywordReply` joins text/Markdown `Contents` with newlines. Media sends each normalized item in order and returns immediately on error. `MessageTarget.SendMarkdown` switches between existing friend/group/channel Markdown calls and channel-DM `SendText`; the three media methods return a descriptive error for channel targets.

- [ ] **Step 4: Verify GREEN and all SDK tests**

Run: `gofmt -w keyword_reply.go keyword_reply_test.go bee_sdk.go && go test ./... -run 'TestParseTextReply|TestMediaItems|TestSendKeywordReply' -v && go vet ./...`

Expected: PASS.

- [ ] **Step 5: Commit reply support**

```powershell
git add keyword_reply.go keyword_reply_test.go bee_sdk.go
git commit -m "feat: dispatch keyword reply types"
```

### Task 5: Runtime Initialization and Message Callback Integration

**Files:**
- Create: `keyword_runtime.go`
- Create: `keyword_runtime_test.go`
- Modify: `plugin_main.go:7-139,244-291`

**Interfaces:**
- Consumes: `RuleStore`, `MatchKeywordRule`, `SendKeywordReply`, and `BeeAPI.MessageTarget` methods.
- Produces: `initializeKeywordRules(*BeeAPI) error`, `currentKeywordStore() *RuleStore`, and `handleKeywordMessage(*BeeAPI, TriggerArea, string, string)`.

- [ ] **Step 1: Write failing runtime tests**

Test the pure coordinator with a fake target: no match makes no call, the first match sends once, send errors are returned, and a degraded outcome is exposed for logging. Also test that `keywordTargetForArea` maps friend/group/channel/channel-private IDs to the correct `MessageTarget.kind`.

- [ ] **Step 2: Run tests to verify RED**

Run: `go test ./... -run 'TestHandleKeyword|TestKeywordTarget' -v`

Expected: FAIL because runtime coordinator functions do not exist.

- [ ] **Step 3: Implement runtime wiring**

Keep the active store behind an RW mutex. `initializeKeywordRules` obtains `bee.GetAppDataDir()`, constructs `keyword_replies.json`, loads it, and publishes the store even when loading returns an error so the settings window can repair the file. Implement a pure helper:

```go
func processKeywordMessage(rules []KeywordRule, target keywordReplyTarget, area TriggerArea, message string) (bool, ReplyOutcome, error) {
    rule, ok := MatchKeywordRule(rules, area, message)
    if !ok { return false, ReplyOutcome{}, nil }
    outcome, err := SendKeywordReply(target, area, rule)
    return true, outcome, err
}
```

Update `PluginName` to `关键词回复插件`, version to `1.0.0`, and description to describe configurable keyword replies. In `onInitialize`, load the store and log errors. In the four message callbacks call `handleKeywordMessage` with `friendID`, `groupID`, `subChannelID`, or `channelID`; log send failures and channel-private Markdown degradation, then return `MessageContinue`.

- [ ] **Step 4: Verify callback integration**

Run: `gofmt -w keyword_runtime.go keyword_runtime_test.go plugin_main.go && go test ./... -run 'TestHandleKeyword|TestKeywordTarget' -v && go test ./...`

Expected: PASS.

- [ ] **Step 5: Commit runtime integration**

```powershell
git add keyword_runtime.go keyword_runtime_test.go plugin_main.go
git commit -m "feat: connect keyword rules to messages"
```

### Task 6: Testable Settings Controller

**Files:**
- Create: `settings_model.go`
- Create: `settings_model_test.go`

**Interfaces:**
- Consumes: `KeywordRule` and `RuleStore`.
- Produces: `RuleDraft`, `NewRuleDraft()`, `DraftFromRule(KeywordRule)`, and `RuleDraft.Rule()`.
- Produces: `SettingsController` methods `Rules`, `Add`, `Update`, `Delete`, and `Move`.

- [ ] **Step 1: Write failing editor-controller tests**

Cover the default case-sensitive checkbox, draft/rule round-trip, duplicate rejection without mutating state, media/channel rejection, delete, upward/downward reordering, persistence after each mutation, and boundary moves that do nothing.

```go
func TestNewRuleDraftDefaultsCaseSensitive(t *testing.T) {
    draft := NewRuleDraft()
    if !draft.CaseSensitive || draft.MatchMode != MatchExact || draft.ReplyType != ReplyText {
        t.Fatalf("unexpected defaults: %+v", draft)
    }
}
```

- [ ] **Step 2: Run tests to verify RED**

Run: `go test ./... -run 'TestNewRuleDraft|TestSettingsController' -v`

Expected: FAIL because the draft and controller do not exist.

- [ ] **Step 3: Implement the controller**

`RuleDraft` uses booleans for the four area checkboxes and one multiline `Content` string. Conversion stores text/Markdown as one `Contents` element and media as `MediaItems([]string{Content})`. Each mutation builds a new slice, calls `store.Replace`, and updates controller memory only after persistence succeeds. `Move(index, delta)` accepts only `-1` or `1` and preserves selection by returning the new index.

- [ ] **Step 4: Verify GREEN**

Run: `gofmt -w settings_model.go settings_model_test.go && go test ./... -run 'TestNewRuleDraft|TestSettingsController' -v`

Expected: PASS.

- [ ] **Step 5: Commit the settings model**

```powershell
git add settings_model.go settings_model_test.go
git commit -m "feat: add keyword settings controller"
```

### Task 7: Native Win32 Rule Editor

**Files:**
- Modify: `settings.go`
- Modify: `plugin_main.go:82-89`

**Interfaces:**
- Consumes: `SettingsController`, `RuleDraft`, and `currentKeywordStore()`.
- Preserves: `showSettingsWindow`, `closeSettingsWindow`, single-instance behavior, topmost positioning, and host-process watcher.

- [ ] **Step 1: Add a failing layout/state test**

Extend `settings_model_test.go` with control-independent list summary expectations:

```go
func TestRuleSummary(t *testing.T) {
    rule := KeywordRule{Keyword: "菜单", MatchMode: MatchFuzzy, Areas: []TriggerArea{AreaFriend, AreaGroup}, ReplyType: ReplyText}
    if got, want := RuleSummary(rule), "菜单｜模糊｜普通消息｜2 个区域"; got != want {
        t.Fatalf("got %q want %q", got, want)
    }
}
```

- [ ] **Step 2: Run the test to verify RED**

Run: `go test ./... -run TestRuleSummary -v`

Expected: FAIL with `undefined: RuleSummary`.

- [ ] **Step 3: Implement summary text and native controls**

Implement `RuleSummary` in `settings_model.go`. Replace the painted template body with standard child controls created during `WM_CREATE`: a left `LISTBOX`; labeled `EDIT` for keyword; `COMBOBOX` controls for match and reply types; checkboxes for case sensitivity and four areas; a multiline scrollable reply `EDIT`; buttons for add/save/delete/up/down; and a status `STATIC`. Use fixed IDs:

```go
const (
    idRuleList = 1001 + iota
    idKeyword
    idMatchMode
    idCaseSensitive
    idAreaFriend
    idAreaGroup
    idAreaChannel
    idAreaChannelPrivate
    idReplyType
    idReplyContent
    idAddRule
    idSaveRule
    idDeleteRule
    idMoveUp
    idMoveDown
    idStatus
)
```

Handle `WM_COMMAND` list selection by loading a draft into controls. Handle add/save/delete/move through `SettingsController`, displaying validation/persistence errors in `idStatus` without clearing inputs. When reply type changes to audio/video/file, uncheck and disable channel checkboxes; re-enable them for text/Markdown. Set window client size to 980×640 at 96 DPI, preserve DPI scaling, system font, stable tab order, center/topmost behavior, and wait-for-close semantics.

Change `showSettingsWindow` to obtain `currentKeywordStore()` and refuse to open with a visible/logged error if initialization has not supplied a store. Keep `onSettings` responsible for logging the open action.

- [ ] **Step 4: Compile and statically verify the Win32 UI**

Run: `gofmt -w settings.go settings_model.go plugin_main.go && go test ./... && go vet ./...`

Expected: PASS.

- [ ] **Step 5: Build the Windows/386 plugin**

Run from Command Prompt with stdin available: `build.bat KeywordReply.dll`

Expected: `build\KeywordReply.dll` exists and is a PE32 DLL; `temp\` and generated root `worker_runtime.go` are cleaned by the script.

- [ ] **Step 6: Commit the native settings window**

```powershell
git add settings.go settings_model.go plugin_main.go
git commit -m "feat: build native keyword rule editor"
```

### Task 8: Documentation and Release Verification

**Files:**
- Modify: `README.md`
- Modify: `docs/AI开发指南.md`
- Modify: `docs/设置窗口开发规范.md`

**Interfaces:**
- Documents: rule order, matching modes, area restrictions, Markdown fallback, `[图片=...]`, media line format, JSON location, and manual Bee checks.

- [ ] **Step 1: Add repository documentation**

Replace template-oriented quick-start examples with the keyword plugin workflow. Include these concrete examples:

```text
关键词：菜单
匹配：精准（大小写敏感）
区域：QQ 好友、群聊
类型：普通消息
内容：请选择功能 [图片=https://example.com/menu.png]
```

```text
C:\media\welcome.mp3
https://example.com/second.mp3
```

State that media sends each non-empty line in order, channel-private Markdown becomes ordinary text, and settings are stored in `plugin_data\关键词回复插件\keyword_replies.json`. Update the settings-window guide with the control layout, inline validation behavior, media-area disabling, immediate persistence, single-instance behavior, and manual keyboard/DPI checks.

- [ ] **Step 2: Run the complete automated verification**

Run: `gofmt -w *.go other/buildmeta/*.go other/worker_runtime.go && go test ./... && go vet ./...`

Expected: PASS with zero test failures and zero vet findings.

- [ ] **Step 3: Run the distributable build and inspect artifacts**

Run: `build.bat KeywordReply.dll`

Expected: build succeeds; `build\KeywordReply.dll` is the only build artifact tracked outside ignored paths, and `git status --short` lists only intended documentation changes before commit.

- [ ] **Step 4: Perform real Bee acceptance checks**

Load the DLL and verify: add/edit/delete/reorder and restart persistence; exact/fuzzy/case behavior; all four trigger areas; first-rule priority; local/URL images; native Markdown in friend/group/channel; Markdown text fallback in channel private; multi-line audio/video/file in friend/group; validation messages; window single-instance behavior; disable closes the window; unload leaves Bee stable.

- [ ] **Step 5: Commit documentation**

```powershell
git add README.md docs/AI开发指南.md docs/设置窗口开发规范.md
git commit -m "docs: explain keyword reply configuration"
```

- [ ] **Step 6: Final review**

Run: `git status --short --branch && git log --oneline --decorate -10`

Expected: clean working tree with separate commits for baseline, design, IPC fix, persistence, matching, replies, runtime, settings controller, native UI, and documentation.
