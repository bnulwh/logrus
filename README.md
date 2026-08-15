# Logrus <img src="http://i.imgur.com/hTeVwmJ.png" width="40" height="40" alt=":walrus:" class="emoji" title=":walrus:"/> [![Build Status](https://github.com/bnulwh/logrus/workflows/CI/badge.svg)](https://github.com/bnulwh/logrus/actions?query=workflow%3ACI) [![Go Reference](https://pkg.go.dev/badge/github.com/bnulwh/logrus.svg)](https://pkg.go.dev/github.com/bnulwh/logrus)

Logrus 是 Go (golang) 的结构化日志库，与标准库 logger 完全 API 兼容。

> **本项目为 Logrus 的维护分支（fork）。** 在保留上游全部功能的基础上，增加了**本地文件系统日志输出**、**纯标准库实现的日志滚动（rotation）**、**简化日志格式（SimpleFormatter）** 等开箱即用的能力，适合中大型应用直接落地使用。

## 特性

- 完全兼容标准库 `log`，可无缝替换 `log` 导入
- 七级日志级别：`Trace`、`Debug`、`Info`、`Warning`、`Error`、`Fatal`、`Panic`
- 结构化日志字段（`WithField` / `WithFields`），告别难以解析的长字符串日志
- 开发环境 TTY 下自动着色输出，非 TTY 下输出 logfmt 兼容格式
- 支持自定义 Formatter 与 Hook 扩展
- ✅ **本地文件系统日志**：`ConfigLocalFileSystemLogger` 一键按级别写入滚动文件
- ✅ **纯标准库滚动写入器**：`RotatingFileWriter` 按时间/大小轮转、自动清理过期文件（无需第三方依赖）
- ✅ **简化格式**：`SimpleFormatter` 输出 `[时间] [级别] [文件:行号:函数()] : 消息`
- ✅ **全局便捷函数**：`IsTraceEnabled`/`IsDebugEnabled`/`IsInfoEnabled` 等日志级别判断
- ✅ 日志文件保留时长可配置：`SetMaxAge` / `GetMaxAge`

## 安装

```bash
go get github.com/bnulwh/logrus
```

注意：请使用**小写**导入路径 `github.com/bnulwh/logrus`（Go 包路径大小写敏感）。

## 快速开始

最简用法——直接使用包级导出的默认 logger：

```go
package main

import (
  log "github.com/bnulwh/logrus"
)

func main() {
  log.WithFields(log.Fields{
    "animal": "walrus",
  }).Info("一只海象出现了")
}
```

因为与标准库完全 API 兼容，你可以把项目中所有 `log` 导入替换为：

```go
import log "github.com/bnulwh/logrus"
```

即可获得 Logrus 的全部灵活性。也可以按需定制：

```go
package main

import (
  "os"
  log "github.com/bnulwh/logrus"
)

func init() {
  // 使用 JSON 格式输出（替代默认的 ASCII 文本格式）
  log.SetFormatter(&log.JSONFormatter{})

  // 输出到 stdout 而不是默认的 stderr
  // 可以是任意 io.Writer，详见下方文件输出示例
  log.SetOutput(os.Stdout)

  // 只记录 warning 及以上级别
  log.SetLevel(log.WarnLevel)
}

func main() {
  log.WithFields(log.Fields{
    "animal": "walrus",
    "size":   10,
  }).Info("一群海象从海面浮现")

  log.WithFields(log.Fields{
    "omg":    true,
    "number": 122,
  }).Warn("这群海象的数量猛增！")

  log.WithFields(log.Fields{
    "omg":    true,
    "number": 100,
  }).Fatal("冰面破裂了！")

  // 常见用法：复用 WithFields 返回的 Entry，避免重复书写公共字段
  contextLogger := log.WithFields(log.Fields{
    "common": "这是公共字段",
    "other":  "这条日志始终会带上它",
  })

  contextLogger.Info("我会带上 common 和 other 字段")
  contextLogger.Info("我也是")
}
```

更高级的用法——同一个应用输出到多个目标时，可以创建独立的 `logrus` Logger 实例：

```go
package main

import (
  "os"
  "github.com/bnulwh/logrus"
)

// 创建新的 logger 实例，可以有任意多个
var log = logrus.New()

func main() {
  // 设置属性的 API 与包级导出的 logger 略有不同，详见 GoDoc
  log.Out = os.Stdout

  // 也可以设置为任意 io.Writer，例如文件：
  // file, err := os.OpenFile("logrus.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
  // if err == nil {
  //   log.Out = file
  // } else {
  //   log.Info("写入文件失败，使用默认 stderr")
  // }

  log.WithFields(logrus.Fields{
    "animal": "walrus",
    "size":   10,
  }).Info("一群海象从海面浮现")
}
```

## 🆕 本地文件系统日志（本分支新增）

调用 `ConfigLocalFileSystemLogger`，日志将按级别分别写入滚动文件，并同时写入一份汇总日志（common），开箱即用：

```go
package main

import (
  log "github.com/bnulwh/logrus"
)

func main() {
  // 日志目录 + 日志文件名（不含扩展名）
  log.ConfigLocalFileSystemLogger("/var/log/myapp", "app.log")
  // 可选：设置日志文件保留时长（默认 7 天）
  log.SetMaxAge(7 * 24 * time.Hour)

  log.Info("写入 info 日志")
  log.Error("写入 error 日志")
}
```

文件布局（按小时滚动，自动保留最新文件软链接）：

```
/var/log/myapp/
├── app.log.log            # 汇总日志（所有级别）→ 指向最新滚动文件
├── app.log.debug.log      # debug 级日志
├── app.log.info.log       # info 级日志
├── app.log.warn.log       # warn 级日志
└── app.log.error.log      # error/fatal/panic 级日志
```

- `debug`/`info`/`warn` 级别同时写入各自的级别文件 + 汇总文件
- `error`/`fatal`/`panic` 级别同时写入 error 文件 + 汇总文件
- 滚动周期：每小时；保留时长：通过 `SetMaxAge` 配置（默认 7 天），过期文件自动清理
- 纯标准库实现，无第三方依赖

### 日志级别判断

```go
if log.IsDebugEnabled() {
  log.Debug("调试模式开启")
}
// 也支持：IsTraceEnabled / IsInfoEnabled / IsWarnEnabled / IsErrorEnabled / IsFatalEnabled / IsPanicEnabled
```

## 🆕 SimpleFormatter（本分支新增）

简洁紧凑的文本格式，自带可选的颜色输出与调用位置信息：

```
[2024-01-01 12:00:00.123] [   info] : 普通信息日志
[2024-01-01 12:00:01.456] [   info] [ main.go : 42 : main() ] : 带调用位置的日志
```

```go
log.SetFormatter(&log.SimpleFormatter{
  Colored: true, // 是否启用颜色
})
// 启用调用位置信息（文件:行号:函数）
log.SetReportCaller(true)
```

## 日志级别

Logrus 有七个日志级别：Trace、Debug、Info、Warning、Error、Fatal 和 Panic。

```go
log.Trace("非常底层的跟踪信息。")
log.Debug("有用的调试信息。")
log.Info("发生了值得注意的事情！")
log.Warn("你应该看看这个。")
log.Error("出错了，但我不打算退出。")
// 记录后调用 os.Exit(1)
log.Fatal("再见。")
// 记录后调用 panic()
log.Panic("我要退出了。")
```

可以设置 `Logger` 的日志级别，只会记录该级别及以上的日志：

```go
// 只记录 info 及以上（warn、error、fatal、panic）。默认值。
log.SetLevel(log.InfoLevel)
```

在调试或 verbose 环境中，设置 `log.Level = logrus.DebugLevel` 会很有用。

## 字段（Fields）

Logrus 鼓励通过结构化字段记录日志，而不是冗长、难以解析的错误消息。例如，与其写 `log.Fatalf("Failed to send event %s to topic %s with key %d")`，不如记录更易于检索的：

```go
log.WithFields(log.Fields{
  "event": event,
  "topic": topic,
  "key":   key,
}).Fatal("发送事件失败")
```

这种 API 会迫使你以产出更有用日志信息的方式思考。`WithFields` 调用是可选的。

### 默认字段（Default Fields）

有时希望在日志语句上始终附加某些字段，例如在请求上下文中始终记录 `request_id` 和 `user_ip`：

```go
requestLogger := log.WithFields(log.Fields{"request_id": request_id, "user_ip": user_ip})
requestLogger.Info("该请求发生了点事情") // 会记录 request_id 和 user_ip
requestLogger.Warn("发生了不太好的事情")
```

### 自动附加字段

除 `WithField` / `WithFields` 添加的字段外，所有日志事件会自动附加：

1. `time` — 条目创建的时间戳
2. `msg` — 日志消息
3. `level` — 日志级别

## Hooks

可以为不同级别添加 Hook。例如在 `Error`、`Fatal`、`Panic` 时发送到异常追踪服务，在 Info 时发送到 StatsD，或同时输出到多个目标（如 syslog）。

本仓库自带 [内置 hooks](hooks/)（`test` / `writer` / `syslog`），也可以编写自定义 hook，在 `init` 中添加：

```go
import (
  log "github.com/bnulwh/logrus"
  logrus_syslog "github.com/bnulwh/logrus/hooks/syslog"
  "log/syslog"
)

func init() {
  hook, err := logrus_syslog.NewSyslogHook("udp", "localhost:514", syslog.LOG_INFO, "")
  if err != nil {
    log.Error("无法连接本地 syslog 守护进程")
  } else {
    log.AddHook(hook)
  }
}
```

> 注：syslog hook 也支持连接本地 syslog（如 `/dev/log`、`/var/run/syslog`、`/var/run/log`），详见 [syslog hook README](hooks/syslog/README.md)。

## Formatters

内置的日志格式器：

- `logrus.TextFormatter` — 如果 stdout 是 tty 则输出带颜色的日志，否则不带颜色。
  - 无 TTY 时强制输出颜色，设置 `ForceColors: true`；有 TTY 时强制不带颜色，设置 `DisableColors: true`。
  - 默认开启颜色时，级别会被截断为 4 个字符。设置 `DisableLevelTruncation: true` 可禁用截断。
  - 输出到 TTY 时，设置 `PadLevelText: true` 可为级别文本添加填充，便于视觉上对齐扫描。
- `logrus.JSONFormatter` — 以 JSON 输出字段，便于 Logstash / Splunk 等解析。
- `logrus.SimpleFormatter` — （本分支新增）紧凑的文本格式，详见上文。

也可以实现 `Formatter` 接口自定义格式器：

```go
type MyJSONFormatter struct {
}

log.SetFormatter(new(MyJSONFormatter))

func (f *MyJSONFormatter) Format(entry *Entry) ([]byte, error) {
  // 注意：这里不包含 Time、Level 和 Message，它们在 Entry 上可直接获取
  serialized, err := json.Marshal(entry.Data)
  if err != nil {
    return nil, fmt.Errorf("Failed to marshal fields to JSON, %w", err)
  }
  return append(serialized, '\n'), nil
}
```

## 记录调用方法名

如需将调用方法作为字段记录，可通过：

```go
log.SetReportCaller(true)
```

这将把调用者作为 `method` 字段添加到日志中。

> 注意：这会带来可测量的开销——在 Go 1.6/1.7 的测试中约为 20%–40%。可以在你的环境中通过基准测试验证：
> ```
> go test -bench=.*CallerTracing
> ```

## Logger 作为 io.Writer

Logrus 可以转换为 `io.Writer`。该 writer 是 `io.Pipe` 的一端，需要由你负责关闭：

```go
w := logger.Writer()
defer w.Close()

srv := http.Server{
  // 创建写入 logrus.Logger 的标准库 log.Logger
  ErrorLog: log.New(w, "", 0),
}
```

写入该 writer 的每一行都会按常规方式输出，经过 formatter 和 hooks 处理，级别为 `info`。

这意味着可以轻松覆盖标准库 logger：

```go
logger := logrus.New()
logger.Formatter = &logrus.JSONFormatter{}

// 让标准库的 log 输出到 logrus
log.SetOutput(logger.Writer())
```

## 测试

Logrus 内置了断言日志消息的设施，通过 `test` hook 实现，提供：

- 为现有 logger 添加装饰器（`test.NewLocal` 和 `test.NewGlobal`）
- 只记录日志消息（不输出任何内容）的测试 logger（`test.NewNullLogger`）

```go
import (
  "testing"

  "github.com/bnulwh/logrus"
  "github.com/bnulwh/logrus/hooks/test"
  "github.com/stretchr/testify/assert"
)

func TestSomething(t *testing.T) {
  logger, hook := test.NewNullLogger()
  logger.Error("Hello error")

  assert.Equal(t, 1, len(hook.Entries))
  assert.Equal(t, logrus.ErrorLevel, hook.LastEntry().Level)
  assert.Equal(t, "Hello error", hook.LastEntry().Message)

  hook.Reset()
  assert.Nil(t, hook.LastEntry())
}
```

## Fatal 处理

Logrus 可以注册一个或多个函数，在记录任何 `fatal` 级别消息时调用。注册的处理函数会在 logrus 执行 `os.Exit(1)` 之前执行。这对于需要优雅关闭的场景很有用。与可以用 `defer recover` 拦截的 `panic("...")` 不同，`os.Exit(1)` 无法被拦截。

```go
handler := func() {
  // 优雅关闭某些东西...
}
logrus.RegisterExitHandler(handler)
```

## 线程安全

默认情况下，Logger 由 mutex 保护以支持并发写入。调用 hooks 和写入日志时都会持有该 mutex。如果确定不需要这种加锁，可以调用 `logger.SetNoLock()` 禁用。

不需要加锁的场景包括：

- 没有注册 hooks，或 hooks 的调用已经是线程安全的
- 写入 `logger.Out` 已经是线程安全的，例如：
  1. `logger.Out` 由锁保护
  2. `logger.Out` 是以 `O_APPEND` 标志打开的 `os.File`，且每次写入小于 4k（支持多线程/多进程写入）

## 许可

本项目采用 [MIT 许可](LICENSE)，与上游 Logrus 一致。
