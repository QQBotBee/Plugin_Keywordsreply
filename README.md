<div align="center">

# Bee 关键词回复

**为 Bee 机器人框架提供可视化配置的关键词自动回复**

顺序规则 · 四类消息区域 · 图片与媒体回复 · 原生 Windows 设置窗口

![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)
![Platform](https://img.shields.io/badge/Platform-Windows-0078D4?logo=windows&logoColor=white)
![Architecture](https://img.shields.io/badge/Architecture-x86%20%2F%2032--bit-555555)
![Build](https://img.shields.io/badge/Build-Go%20%2B%20Zig-F7A41D?logo=zig&logoColor=white)
![Encoding](https://img.shields.io/badge/Bee-GBK%20%2F%20CP936-blueviolet)

[快速开始](#快速开始) · [规则行为](#规则行为) · [架构说明](#架构说明) · [开发文档](#开发文档)

</div>

---

## 项目简介

Bee 关键词回复允许管理员在原生 Windows 设置窗口中维护自动回复规则，无需修改源码。规则支持精准或模糊匹配、大小写策略、四种触发区域和五种回复类型，并按列表顺序只执行第一条命中规则。

插件沿用“**纯 C Bee 壳 DLL + 独立 Go Worker**”架构：Bee 进程只加载纯 C DLL，Go 业务运行在独立进程中，避免 Go Runtime 随 DLL 被 `FreeLibrary()` 卸载时影响宿主稳定性。

最终构建产物仍然只有一个 DLL。Go Worker 会作为资源内嵌在 DLL 中，由插件在运行时自动释放和管理。

```text
单个插件 DLL
├── 纯 C Bee 壳
│   ├── 11 个 stdcall 中文导出
│   ├── 插件生命周期管理
│   ├── Worker 释放与进程管理
│   ├── GBK / UTF-8 编码转换
│   └── robot.api 进程内调用
└── 内嵌 Go Worker
    ├── Go 插件业务
    ├── Bee Go SDK
    ├── 原生设置窗口
    └── JSON Lines IPC
```

## 核心特性

- **可配置规则列表**：新增、编辑、删除和上下移动后立即持久化。
- **确定性匹配**：精准/模糊、大小写敏感/不敏感，严格按列表顺序取第一条命中规则。
- **四种触发区域**：QQ 好友、群聊、频道消息和频道私信。
- **五种回复类型**：普通消息、原生 Markdown、语音、视频和文件。
- **图文普通消息**：支持一个 `[图片=本地路径或网址]` 标记。
- **媒体批量回复**：每个非空行作为一个媒体项按顺序发送，失败时停止剩余发送。
- **Go 编写插件业务**：生命周期、消息回调和事件处理均可使用 Go 实现。
- **单 DLL 交付**：Go Worker 作为资源内嵌，无需额外分发 EXE 或运行库。
- **安全加载与卸载**：Go Runtime 不进入 Bee 宿主进程，规避热卸载风险。
- **完整生命周期**：支持初始化、启用、禁用、卸载和设置入口。
- **完整消息回调**：支持频道私信、频道消息、频道事件、好友私聊、群聊消息和通常事件。
- **具名业务参数**：直接使用 `groupID`、`friendID`、`messageID` 等变量，无需操作 IPC 数组下标。
- **完整 Bee API**：包含 1～72 操作码、常用链式接口及按钮响应快捷方法。
- **完整事件常量**：提供 32 个事件常量及中文说明。
- **编码自动转换**：自动处理 Bee 的 GBK/CP936 与 Go UTF-8 编码边界。
- **专属数据目录**：通过 `GetAppDataDir()` 获取当前插件的数据目录。
- **原生设置窗口**：支持单实例、固定尺寸、居中显示和始终置顶。
- **一键构建**：使用 `build.bat` 自动生成元数据、编译 Worker、嵌入资源并链接 DLL。

## 环境要求

| 环境/工具 | 要求 | 获取方式 |
| --- | --- | --- |
| Windows | Windows 10/11 | — |
| Go | 1.22 或更高版本 | [官方下载](https://go.dev/dl/) |
| Zig | 已加入系统 `PATH` | [官方下载](https://ziglang.org/download/) |
| Bee | 支持 32 位 stdcall 插件 | Bee 官方渠道 |

构建脚本仅依赖：

- Windows CMD
- Go
- Zig

无需安装 MinGW、Python、Node.js 或 PowerShell。

## 快速开始

### 1. 构建插件

在 Windows CMD 中执行：

```bat
build.bat KeywordReply.dll
```

构建成功后，将生成：

```text
build\KeywordReply.dll
```

### 2. 加载并打开设置

在 Bee 插件管理器中加载 DLL、启用插件，然后点击“设置”。左侧是按优先级排列的规则列表，右侧用于编辑当前规则。

### 3. 添加第一条规则

填写以下内容后点击“新增”：

```text
关键词：菜单
匹配：精准（大小写敏感）
区域：QQ 好友、群聊
类型：普通消息
内容：请选择功能 [图片=https://example.com/menu.png]
```

收到“菜单”时，插件发送文字和图片。普通消息最多允许一个 `[图片=...]` 标记，地址可以是本地路径或网址；空地址或多个标记无法保存。

### 4. 配置媒体回复

语音、视频和文件内容每行填写一个本地路径或网址，例如：

```text
C:\media\welcome.mp3
https://example.com/second.mp3
```

插件忽略空行，并按顺序发送每个非空行；任一项失败后停止本规则的剩余发送。媒体回复只支持 QQ 好友和群聊，选择媒体类型后频道区域会自动取消并禁用。

### 5. 配置位置

规则立即保存到 UTF-8 JSON：

```text
Bee框架根目录\plugin_data\关键词回复\keyword_replies.json
```

设置窗口关闭或 Bee 重启后规则仍然保留。不建议在插件运行期间手工编辑该文件。

## 规则行为

- 精准匹配：去除收到消息首尾空白后，与关键词完整相等。
- 模糊匹配：去除首尾空白后，消息任意位置包含关键词。
- 大小写策略按每条规则独立配置，默认大小写敏感。
- 规则从列表顶部向下扫描，只发送第一条命中规则。
- 普通消息和 Markdown 支持全部四个区域。
- 频道私信没有原生 Markdown 接口，因此自动按普通文字发送并记录降级日志。
- 语音、视频和文件仅支持 QQ 好友与群聊。
- 所有消息回调均返回 `MessageContinue`，不会拦截其他插件。

## 开发插件

### 生命周期入口

| 回调 | 触发时机 | 推荐用途 |
| --- | --- | --- |
| `onInitialize` | `Bee_初始化` 返回插件信息之前 | 创建数据目录、读取配置、准备基础资源 |
| `onEnable` | 插件被启用 | 启动定时任务、监听器和运行期服务 |
| `onDisable` | 插件被禁用 | 停止任务、释放运行期资源、关闭设置窗口 |
| `onUnload` | 插件被卸载 | 最终清理和卸载日志 |
| `onSettings` | 用户点击“设置” | 打开或聚焦设置窗口 |

生命周期顺序：

```text
Bee_初始化
└── onInitialize
    └── 返回插件信息

Bee_插件被启用
└── onEnable

Bee_插件被禁用
└── onDisable
    ├── 停止运行任务
    └── 关闭设置窗口

Bee_卸载
└── onUnload
    └── 最终清理
```

> Bee 只允许禁用后的插件被卸载，因此 `onUnload` 不重复关闭设置窗口。

### 消息与事件入口

| 回调 | 用途 |
| --- | --- |
| `onChannelPrivate` | 收到频道私信消息 |
| `onChannelMessage` | 收到频道消息 |
| `onChannelEvent` | 收到频道事件 |
| `onPrivateMessage` | 收到好友私聊消息 |
| `onGroupMessage` | 收到群聊消息 |
| `onCommonEvent` | 收到好友、私聊或群聊通常事件 |

所有业务入口均使用具名参数，并附有中文说明。例如：

```go
func onGroupMessage(
    robotJSON string, // 当前机器人上下文 JSON，用于创建本次消息的 BeeAPI
    groupID string,   // 来源群 ID
    userID string,    // 触发人 ID，即群消息发送人
    message string,   // 收到的群聊消息内容
    messageID string, // 消息 ID，用于撤回、引用等上下文相关 API
) int {
    bee, err := NewBeeAPI(robotJSON)
    if err != nil {
        return MessageContinue
    }

    if message == "菜单" {
        _, _ = bee.Group(groupID).SendText("请选择功能")
    }

    return MessageContinue
}
```

### 消息处理结果

```go
return MessageContinue  // 继续投递给后续插件
return MessageIntercept // 拦截消息，其他插件无法接收到
return MessageIgnore    // 与 MessageContinue 同值，保留 Bee SDK 语义别名
```

## SDK 示例

### 输出日志

```go
_ = bee.Log("插件正在运行")
```

### 发送好友消息

```go
_, _ = bee.Friend(friendID).SendText("你好")
```

### 发送群聊消息

```go
_, _ = bee.Group(groupID).SendText("大家好")
```

### 发送频道消息

```go
_, _ = bee.Channel(subChannelID).SendText("频道消息")
```

### 发送频道私信

```go
_, _ = bee.ChannelDM(channelID).SendText("频道私信")
```

### 响应按钮事件

```go
case EventInteractionCreate:
    _, _ = bee.RespondButton(0)
```

`RespondButton()` 会自动使用当前回调中的 `event_id`。

### 获取应用数据目录

```go
dataDir, err := bee.GetAppDataDir()
if err != nil {
    return
}

configPath := filepath.Join(dataDir, "config.json")
```

生产环境返回：

```text
Bee框架根目录\plugin_data\插件名称
```

插件配置、数据库、缓存和文件日志都应写入该目录。不要把持久化数据写入 `plugin`、`temp_plugin` 或 DLL 所在目录。

## 上下文使用规则

`BeeAPI` 保存创建时的机器人上下文，其中包含 `msg_id`、`event_id` 和 `plugin_id`。

- 回复、引用、撤回和按钮响应等操作依赖当前上下文，必须使用本次回调创建的 `BeeAPI`。
- 输出日志、取框架信息等框架级操作不依赖消息上下文，可以复用仍然有效的 API 对象。
- 不要缓存上一条消息的 `BeeAPI`，再用于下一条消息的回复、引用、撤回或按钮响应。

## 项目结构

```text
.
├── README.md                   # 项目说明
├── plugin_main.go              # 插件信息、生命周期和业务回调
├── bee_sdk.go                  # Bee Go SDK、事件常量和 IPC 客户端
├── keyword_config.go           # 规则模型、校验和 JSON 持久化
├── keyword_match.go            # 顺序匹配引擎
├── keyword_reply.go            # 回复解析与发送分发
├── keyword_runtime.go          # 初始化和消息回调接线
├── settings_model.go           # 可测试的设置控制器
├── settings.go                 # Windows 原生规则编辑器
├── build.bat                   # Windows 一键构建脚本
├── go.mod
├── go.sum
├── docs/
│   ├── API参考.md              # 1～72 API 与数据目录说明
│   ├── AI开发指南.md           # 架构边界与开发规则
│   └── 设置窗口开发规范.md     # 设置窗口开发约束
└── other/
    ├── bee_bridge.c            # 纯 C Bee 壳、生命周期与双向 IPC
    ├── BeePlugin.def           # 11 个 GBK 中文导出名
    ├── worker_runtime.go       # Worker 入口、IPC 解码与事件分发
    └── buildmeta/
        └── main.go             # 元数据与构建源码生成器
```

`other/` 保存底层桥接实现，不应为关键词规则功能修改其中的 ABI 或 IPC 协议。

## 架构说明

### 为什么不直接把 Go 编译进 DLL？

Bee 加载插件时会将 DLL 复制到 `temp_plugin`，再使用 `LoadLibraryA` 加载。插件卸载时，Bee 会调用 `FreeLibrary()` 真正卸载 DLL。

Go Runtime 不适合在同一宿主进程中被反复动态加载和卸载。即使插件代码已经关闭窗口和业务任务，直接嵌入 Go Runtime 的 DLL 仍可能在卸载后导致 Bee 崩溃。

本模板将两部分彻底分离：

```text
Bee.exe
└── 纯 C 插件 DLL
    ├── 32 位 stdcall ABI
    ├── Worker 生命周期
    ├── 编码转换
    └── robot.api 调用

独立 Go Worker
├── 插件业务
├── Bee Go SDK
├── 设置窗口
└── IPC
```

Bee 可以安全卸载纯 C DLL，Go Worker 则通过正常进程退出完成 Go Runtime 清理。

### IPC 工作方式

C 壳与 Worker 使用标准输入输出上的 JSON Lines 通信：

```text
Bee 回调
→ C 壳将 GBK 参数编码为 Base64
→ Worker 解码并转换为 UTF-8
→ 调用 Go 业务回调
→ Worker 发出 api_call
→ C 壳在 Bee 进程内调用 robot.api
→ 返回 api_result
→ Worker 返回 event_result
```

C 壳等待 `event_result` 时仍会处理嵌套的 `api_call`，避免同步 SDK 调用发生死锁。

## 编码与构建约束

- 构建目标固定为 `Windows/386`。
- Bee 文本边界使用 GBK/CP936，Go 内部使用 UTF-8。
- `build.bat` 必须保存为 GBK/CP936，并使用 CRLF 换行。
- `other/BeePlugin.def` 必须保持 GBK 编码。
- 不要将 `cgo`、`buildmode=c-archive` 或 Go Runtime 放回 Bee DLL。
- 不要提交 `build/`、`temp/`、DLL、EXE、RES、LIB 或 PDB 等构建产物。

## 开发文档

- [API 参考](docs/API参考.md)
- [AI 开发指南](docs/AI开发指南.md)
- [设置窗口开发规范](docs/设置窗口开发规范.md)

## 验证范围

交叉编译可以验证 Worker 和 DLL 是否为 Windows PE32，但以下功能必须在真实 Windows/Bee 环境中测试：

- 插件加载、初始化、启用、禁用和卸载；
- 四个消息区域的关键词匹配与第一规则优先级；
- 图片、Markdown 降级及多行媒体发送；
- 设置窗口增删改排、即时持久化、单实例、键盘导航和 DPI；
- Worker 在 Bee 宿主退出后的自动清理；
- 插件重复加载和卸载的稳定性。

## 常见问题

### 构建时提示找不到 Go 或 Zig

确认 `go.exe` 和 `zig.exe` 所在目录已经加入系统 `PATH`，重新打开 CMD 后执行：

```bat
go version
zig version
```

### 中文提示或插件名乱码

不要把 `build.bat` 转换为 UTF-8。脚本使用：

```bat
chcp 936
```

因此文件必须保持 GBK/CP936 编码和 CRLF 换行。

### 应用数据应该写在哪里？

统一使用：

```go
dataDir, err := bee.GetAppDataDir()
```

不要根据 DLL 路径或当前工作目录自行拼接数据目录。

### 为什么最终只有一个 DLL？

Go Worker 已作为资源嵌入 DLL。插件首次初始化或运行时，C 壳会自动释放并启动 Worker。

## 贡献指南

欢迎提交 Issue 和 Pull Request。

提交改动前请确保：

1. 保持 32 位 stdcall ABI 不变；
2. 保持 GBK/UTF-8 编码边界正确；
3. 不把 Go Runtime 重新链接进 Bee DLL；
4. Windows/386 Worker 和纯 C 壳 DLL 均能成功构建；
5. 生命周期、消息回调和设置窗口改动已在真实 Bee 中验证；
6. 不提交任何构建产物或临时文件。

## 发布检查清单

- [ ] `go test ./...` 与 `go vet ./...` 通过
- [ ] `build.bat` 构建成功
- [ ] `build\KeywordReply.dll` 是 PE32 i386 DLL
- [ ] 插件可以在 Bee 中初始化和启用
- [ ] 精准、模糊、大小写和首条规则优先级已验证
- [ ] 好友、群聊、频道消息、频道私信已验证
- [ ] 图片、Markdown 降级、语音、视频和文件已验证
- [ ] 规则增删改排和重启持久化已验证
- [ ] 禁用后 Worker 正常退出
- [ ] 卸载后 Bee 不崩溃
- [ ] 设置窗口单实例、键盘操作、DPI 和关闭行为正常
- [ ] 配置写入 `plugin_data\关键词回复\keyword_replies.json`

---

<div align="center">

如果这个项目对你有帮助，欢迎提交 Issue、Pull Request，或为项目点一个 Star。

</div>
