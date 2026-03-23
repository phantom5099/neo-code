# 🛡️ 安全拦截模块 (Security Interceptor) 测试手册

## 1. 模块概述
安全拦截模块是 NeoCode 的核心安全防线，负责审计 AI 试图执行的所有敏感操作（读文件、写文件、执行命令、网络请求）。它采用 **黑名单、白名单、黄名单** 三级过滤机制，并具备自动化的路径规范化防御能力。

## 2. 核心安全机制
### 2.1 三级名单逻辑
- **黑名单 (Blacklist)**: 优先级最高。匹配成功则立即 `ActionDeny`（拒绝）。
- **白名单 (Whitelist)**: 优先级中等。匹配成功则 `ActionAllow`（允许）。
- **黄名单 (Yellowlist)**: 优先级最低。匹配成功则 `ActionAsk`（询问用户）。
- **默认兜底**: 若均未匹配，默认执行 `ActionAsk`。

### 2.2 路径规范化 (Path Normalization)
为防止 AI 通过构造特殊路径绕过拦截，模块在匹配前会执行以下操作：
- **Cleaning**: 消除 `../`、`./` 以及冗余斜杠（如 `///`）。
- **Slash Uniformity**: 统一将 Windows 的反斜杠 `\` 转换为正斜杠 `/`，确保跨平台兼容。
- **Cross-Domain Prevention**: 严禁路径以 `..` 开头，防止 AI 访问工作目录外的系统文件。

## 3. 测试用例设计

### 3.1 基础匹配测试
| 场景 | 工具类型 | 输入目标 | 预期结果 | 说明 |
| :--- | :--- | :--- | :--- | :--- |
| 黑名单拦截 | Read | `.git/config` | `Deny` | 禁止访问敏感 Git 配置 |
| 白名单允许 | Read | `src/main.go` | `Allow` | 允许正常阅读源码 |
| 黄名单询问 | Write | `src/main.go` | `Ask` | 修改代码需经用户确认 |
| 命令拦截 | Bash | `rm -rf /` | `Deny` | 禁止高危删库命令 |

### 3.2 对抗性绕过测试 (Security Focus)
| 场景 | 输入目标 | 处理后路径 | 预期结果 | 防御原理 |
| :--- | :--- | :--- | :--- | :--- |
| 路径穿越绕过 | `src/../.git/config` | `.git/config` | `Deny` | `filepath.Clean` 消除回溯符 |
| 冗余斜杠绕过 | `.git////config` | `.git/config` | `Deny` | 消除多余分隔符 |
| 跨工作目录攻击 | `../../etc/passwd` | `../../etc/passwd` | `Deny` | 识别并拦截 `..` 前缀 |
| 平台差异绕过 | `src\.git\config` | `src/.git/config` | `Deny` | 统一转换为正斜杠匹配 |

### 3.3 复杂逻辑测试
- **通配符测试**: 验证 `**/*.go` 是否能正确匹配深层目录。
- **域名匹配**: 验证 `*.google.com` 是否能允许子域名请求。
- **未知工具**: 验证传入非法 `toolType` 时，系统是否能安全地回退到 `Ask` 模式。

## 4. 自动化测试运行指南

### 4.1 环境准备
确保已安装 Go 环境，并处于 `test1` 目录下。

### 4.2 执行单元测试
运行以下命令执行专门的安全测试用例：
```bash
go test -v internal/server/service/security_service_test.go internal/server/service/security_service.go
```

### 4.3 查看测试覆盖率
若要查看安全模块的测试覆盖情况：
```bash
go test -coverprofile=cover.out ./internal/server/service/...
go tool cover -html=cover.out
```

## 5. 维护建议
1. **规则同步**: 修改 `configs/security/` 下的 YAML 文件后，务必运行单元测试验证规则是否生效。
2. **正则优化**: 对于 `Bash` 命令的正则匹配，应遵循“最小特权原则”，避免编写过于宽泛的匹配模式。
3. **日志审计**: 在生产环境中，所有被 `Deny` 的操作都应记录在案，以便进行安全回溯。
