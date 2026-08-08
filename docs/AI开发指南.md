# Bee Go SDK：AI 开发指南

## 架构边界

```text
Bee.exe
  └─ 纯 C BeeGoPlugin.dll
       ├─ 11 个 stdcall 中文导出
       ├─ worker 生命周期
       ├─ GBK/UTF-8 边界
       └─ robot.api 进程内调用

bee_go_worker.exe
  ├─ plugin_main.go 业务回调
  ├─ bee_sdk.go 完整 1～72 API
  └─ JSON Lines IPC
```

禁止把 Go runtime、cgo 或 `buildmode=c-archive` 放回 Bee DLL。

## 业务代码位置

关键词回复功能按职责分为：

- `keyword_config.go`：规则枚举、校验、深拷贝快照和原子 JSON 持久化。
- `keyword_match.go`：区域过滤、精准/模糊匹配和首条规则优先。
- `keyword_reply.go`：图片标记解析、Markdown 和媒体分发。
- `keyword_runtime.go`：规则初始化、目标映射和四类消息回调协调。
- `settings_model.go`：不依赖 Win32 的编辑控制器。
- `settings.go`：原生 Win32 控件和窗口生命周期。

每次回调都必须使用当前机器人 JSON 创建新的 `BeeAPI`。不要缓存上一条消息的上下文，因为 `msg_id`、`event_id`、`plugin_id` 属于单次回调。

## 规则约束

- 关键词唯一，列表顺序就是匹配优先级，只执行第一条命中规则。
- 精准匹配比较去除首尾空白后的完整消息；模糊匹配执行包含判断。
- 大小写敏感默认开启，并按规则独立配置。
- 触发区域为 `friend`、`group`、`channel`、`channel_private`。
- 普通消息和 Markdown 支持全部区域；频道私信 Markdown 降级为普通文字。
- 语音、视频和文件只允许好友与群聊，每个非空内容行按顺序发送。
- 普通消息最多包含一个 `[图片=路径或网址]` 标记。

配置文件固定为：

```text
plugin_data\关键词回复插件\keyword_replies.json
```

保存必须通过 `RuleStore.Replace`，不要从窗口代码直接写文件。控制器只有在原子替换成功后才能更新内存规则。

## 常用 API

```go
bee, err := beeFromArgs(args)
if err != nil {
    return MessageContinue
}

_, _ = bee.Friend(friendID).SendText("你好哦")
_, _ = bee.Group(groupID).SendImage("说明", imageURL)
_, _ = bee.Channel(channelID).Reply(messageID, "收到", "")
_, _ = bee.ChannelDM(guildID).SendText("私信回复")
```

复杂参数使用 `bee.ctx` 上的完整方法；完整操作码见 `API参考.md`。

## IPC 纪律

1. worker stdout 只能输出 JSON Lines IPC，日志写 stderr 或插件数据目录。
2. worker 发出 `api_call` 后同步等待匹配 ID 的 `api_result`。
3. C 壳等待 `event_result` 时必须处理中途出现的 `api_call`，否则同步 SDK 调用会死锁。
4. Go 进程不得直接调用机器人 JSON 中的 `api` 地址；函数地址只在 Bee 进程有效。
5. 参数不能包含 `%@#bee#@%`，原 Bee 协议没有转义机制。

## 生命周期

- `onInitialize` 是 Go 业务初始化入口，由 C 壳在 `Bee_初始化` 返回插件信息之前调用一次；适合读取配置和准备基础资源，不做耗时业务或长期任务。
- `pluginMetadata` 只描述插件名称、作者、版本和说明，不是初始化入口。
- 启用时启动任务。
- 禁用时关闭任务和设置窗口。
- 卸载时完成最终清理。
- worker 还会在宿主 PID 消失或 stdin EOF 时退出。

## 验证边界

自动验证运行 `gofmt -w *.go other/buildmeta/*.go other/worker_runtime.go`、`go test ./...`、`go vet ./...` 和 `build.bat KeywordReply.dll`。真实 Bee 还必须检查规则增删改排和重启持久化、四个区域、精准/模糊/大小写、首条优先、图片、Markdown 降级、多行媒体、窗口单实例、禁用关闭以及卸载稳定性。
