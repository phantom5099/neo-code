# 🛡️ NeoCode 安全拦截器 (Security Interceptor) 模块文档

## 1. 接口提供与调用规格说明

本模块为 Agent 工具执行层（Executor）提供统一的底层安全校验接口。所有具有潜在副作用的工具（如文件读写、终端执行、网络请求）在正式执行前，**必须**同步调用此接口获取权限批文。

### 1.1 核心状态枚举 (`Action`)

拦截器会返回以下三种明确的决策动作：

- `ActionDeny` ("deny"): **拒绝执行**。触发黑名单或安全策略，底层必须停止操作并向模型返回错误。
- `ActionAllow` ("allow"): **静默放行**。命中白名单，可直接执行无需用户干预。
- `ActionAsk` ("ask"): **请求确认**。命中黄名单或未匹配任何规则，必须挂起工作流并请求用户授权。

### 1.2 对外暴露接口 (`SecurityService`)

```Go
// SecurityService 接口定义
type SecurityService interface {
	Check(toolType string, target string) domain.Action
}
```

**参数规范：**

- `toolType` (string): 必须为以下四种标准工具类型之一：
  - `"Read"`: 读取本地文件。
  - `"Write"`: 写入或修改文件。
  - `"Bash"`: 执行 Shell 命令。
  - `"WebFetch"`: 发起外部网络请求。
- `target` (string): 操作的目标（路径、命令或域名）。

---

## 2. 模块架构与目录结构

### 2.1 架构概述

本模块采用**“防御性路径预处理”**与**“三态漏斗过滤”**架构。在匹配规则前，先通过规范化引擎消除路径绕过风险，随后依次通过黑、白、黄名单进行判决。

### 2.2 目录结构 (test1 项目)

```Plaintext
test1/
├── configs/security/           # [配置层] YAML 规则文件
│   ├── blacklist.yaml          # 绝对禁区 (Deny)
│   ├── whitelist.yaml          # 信任区域 (Allow)
│   └── yellowlist.yaml         # 需确认区域 (Ask)
├── internal/server/
│   ├── domain/
│   │   └── security.go         # 核心接口与数据结构定义
│   ├── service/
│   │   ├── security_service.go # 拦截引擎实现 (包含清洗与匹配逻辑)
│   │   └── security_service_test.go # 90.6% 覆盖率的自动化测试
```

---

## 3. 核心安全防御机制 (Security Hardening)

模块在规则匹配前引入了主动防御逻辑，专门应对**对抗性输入 (Adversarial Inputs)**。

### 3.1 路径规范化 (Normalization)
针对 `Read` 和 `Write` 操作，系统会自动执行：
- **`filepath.Clean`**: 消除 `./`、多余斜杠以及 `../` 回溯符。将 `src/../.git/config` 强行转化为 `.git/config`。
- **`filepath.ToSlash`**: 将 Windows 的 `\` 统一为 `/`，防止利用平台差异逃逸。

### 3.2 跨域主动拦截
- **边界防御**: 若清洗后的路径以 `..` 开头（意图跳出当前项目工作目录），拦截器会跳过所有名单逻辑，**直接返回 `ActionDeny`**。

---

## 4. 自动化测试与质量保证

本模块通过了严苛的自动化测试，核心逻辑（`security_service.go`）的 **测试覆盖率达到 90.6%**。

### 4.1 测试场景覆盖
- **基础匹配**: 黑名单命中、白名单放行、黄名单询问。
- **对抗性绕过**: 模拟路径穿越（Traversal）、冗余斜杠、跨目录攻击。
- **通配符深度**: 验证 `**/*.go` 对多级目录的递归匹配。
- **网络域名**: 验证对 `*.domain.com` 子域名的通配支持。

### 4.2 运行测试
```bash
go test -v internal/server/service/security_service.go internal/server/service/security_service_test.go
```

---

## 5. 具体实现原理

### 5.1 优先级漏斗判决 (Funnel Priority)
1. **最高级 (Hard-Deny)**: 路径跨域或命中黑名单规则。
2. **第二级 (Allow)**: 命中白名单规则。
3. **第三级 (Ask)**: 命中黄名单规则。
4. **兜底 (Default)**: 未匹配任何规则，降级为 `ActionAsk`（零信任原则）。

### 5.2 匹配引擎双重分流
- **文件系统语义 (doublestar)**: 处理 `Read/Write`，支持真正的 `**` 跨目录匹配。
- **纯文本正则语义 (regexp)**: 处理 `Bash/WebFetch`，通过 `regexp.QuoteMeta` 防止正则注入，并将 `*` 安全映射为命令通配符。
