# NeoCode 工具安全拦截说明

## 目标

NeoCode 会在执行存在副作用的工具之前进行安全判断。这个安全层属于 `tool` 模块的横切能力，当前实现位于：

- `internal/tool/security/`
- `configs/security/`

## 三种动作

- `deny`：直接拒绝执行
- `allow`：静默放行
- `ask`：挂起执行，等待用户确认

## 当前实现位置

```text
configs/security/
  blacklist.yaml
  whitelist.yaml
  yellowlist.yaml

internal/tool/
  bash.go
  edit.go
  grep.go
  list.go
  read.go
  write.go
  security.go
  protocol/
  security/
```

其中：

- `internal/tool/security/loader.go` 负责加载 YAML 规则。
- `internal/tool/security/checker.go` 负责规则匹配与优先级判定。
- `internal/tool/security.go` 负责运行时挂载 checker，以及一次性批准 `ask` 动作。

## 规则优先级

执行顺序如下：

1. 黑名单命中：`deny`
2. 白名单命中：`allow`
3. 黄名单命中：`ask`
4. 未命中任何规则：默认 `ask`

## 路径与命令处理

### 文件类工具

对于 `read / write` 等文件操作，系统会先做路径规范化，并禁止跳出当前 workspace。

### Bash

对于 `bash`，当前使用通配规则匹配命令文本，并在运行前由安全层先做判定。

## 为什么从根目录平铺拆出去

之前 `tool` 根目录同时放了：

- `security_service.go`
- `security_config_repository.go`
- `security_types.go`

这会把“工具本体”和“横切策略”混在一起，也容易让单实现模块被过度拆分。

现在改成 `internal/tool/security/` 后：

- 目录职责更清楚
- 测试边界更稳定
- TUI / runtime 不需要了解安全层内部结构

## 测试

运行：

```bash
go test ./internal/tool/...
```

重点覆盖：

- 黑白黄名单匹配
- 路径穿越拦截
- bash ask / deny 行为
- 一次性批准后的继续执行
