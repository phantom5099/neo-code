# 📄 NeoCode API 契约书与开发指南 (v1.0)

## 1. 概述 (Overview)
为了实现 NeoCode 项目的前后端解耦，我们引入了基于 **Protocol Buffers (Protobuf)** 的 API 契约层。该契约定义了前端（TUI）与后端（Server）通信的标准格式，是项目从“本地调用版”向“网络分布式版”演进的核心基石。

**核心原则**：
- **契约先行**：所有字段变更必须先修改 `.proto` 文件。
- **逻辑隔离**：`api/proto` 目录下的代码仅作为数据标准，不包含业务逻辑。
- **双向测试**：前端与后端可基于此契约进行独立 Mock 测试。

---

## 2. 现阶段 API 契约详述

契约原稿位于：`api/proto/chat.proto`

### 2.1 核心消息结构
| 消息名称 | 说明 | 关键字段 |
| :--- | :--- | :--- |
| `Message` | 单条对话条目 | `role` (角色), `content` (正文) |
| `ChatRequest` | 前端发起的请求 | `model` (模型ID), `messages` (历史列表) |
| `ChatResponse` | 后端回传的响应 | `content` (正文片段), `is_finished` (结束标识) |
| `Status` | **错误处理块** | `code` (状态码, 0为成功), `message` (错误描述) |
| `ResponseMetadata` | **元数据块** | `model_name` (实际模型), `usage_tokens` (Token统计) |

### 2.2 命名规范
- **Proto 文件**：使用 `snake_case`（下划线命名），如 `user_input`。
- **生成的 Go 代码**：自动转为 `PascalCase`（首字母大写），如 `UserInput`，以符合 Go 的导出规则。

---

## 3. 《环境安装极简指南》

为了运行自动化脚本生成代码，组员需完成以下三步配置：

### 第一步：下载并安装 Protoc 编译器
1.  **下载**：访问 [Protobuf Releases](https://github.com/protocolbuffers/protobuf/releases)。
2.  **选择**：Windows 用户请下载 `protoc-xx.x-win64.zip`。
3.  **配置**：解压并将 `bin` 目录（内含 `protoc.exe`）的路径添加到系统的 **环境变量 Path** 中。
4.  **验证**：打开终端输入 `protoc --version`，看到版本号即成功。

### 第二步：安装 Go 语言插件
在终端执行以下命令，让编译器支持生成 Go 代码：
```powershell
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
```
*注：请确保 `$GOPATH/bin`（通常是 `C:\Users\用户名\go\bin`）也在 Path 环境变量中。*

### 第三步：同步运行时依赖
在 `test1` 项目根目录下执行：
```powershell
go get google.golang.org/protobuf
```

---

## 4. 自动化工具链使用

我们提供了快捷脚本，组员无需记忆复杂的编译命令。

- **脚本位置**：`scripts/gen_proto.ps1`
- **使用方法**：在 `test1` 目录下运行：
  ```powershell
  ./scripts/gen_proto.ps1
  ```
- **输出结果**：成功后会自动在 `api/proto/` 目录下更新 `chat.pb.go` 文件。**请勿手动修改生成的 .pb.go 文件。**

---

## 5. 前后端分离测试指引

有了契约层，前后端可以“背靠背”工作：

### 5.1 后端组员：验证契约兼容性
后端同学无需启动 TUI 界面，只需编写单元测试：
1.  手动构造 `proto.ChatRequest` 结构体。
2.  编写转换函数将 `proto` 对象转为 `domain` 对象。
3.  验证 Service 返回的数据是否能填入 `proto.ChatResponse`。
*参考示例：`test/contract_test.go`*

### 5.2 前端组员：独立 UI Mock 测试
前端同学无需配置后端的 API Key 或 LLM 环境：
1.  在测试代码中引入 `go-llm-demo/api/proto` 包。
2.  手动构造各种 `proto.ChatResponse` 假数据（包含成功的、报错的、包含元数据的）。
3.  直接将假数据喂给 TUI 的 `update` 逻辑，调试流式高亮和错误提示框。

---

## 6. 架构师的温馨提示

1.  **关于红线报错**：如果 IDE 中生成的 `pb.go` 文件有红线，请运行 `go mod tidy`。
2.  **版本一致性**：请确保团队内部的 `protoc` 版本差异不要太大（建议 34.1）。
3.  **契约的法律效力**：一旦 `chat.proto` 经过协商合并入主分支，任何破坏契约的字段修改都应被视为 **Breaking Change**。

---
*NeoCode 架构组 2026-03-23*
