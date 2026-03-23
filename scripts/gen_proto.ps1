# NeoCode API 契约自动化生成脚本
# 确保你已经安装了 protoc 工具以及 protoc-gen-go 插件

# 获取当前脚本所在目录
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$RootDir = Resolve-Path (Join-Path $ScriptDir "..")
$ProtoDir = Join-Path $RootDir "api/proto"
$ProtoFile = Join-Path $ProtoDir "chat.proto"

Write-Host "--- 开始生成 API 契约代码 ---" -ForegroundColor Cyan

# 检查 protoc 是否存在
if (-not (Get-Command "protoc" -ErrorAction SilentlyContinue)) {
    Write-Host "失败：未在系统中找到 protoc 编译器。" -ForegroundColor Red
    Write-Host "请参考文档安装：https://grpc.io/docs/protoc-installation/" -ForegroundColor Gray
    exit 1
}

# 执行 protoc 命令
# --proto_path 指定搜索 .proto 文件的目录
# --go_out 控制生成结构体代码 (.pb.go) 的根目录
# --go_opt=module=go-llm-demo 告诉工具按照 go.mod 中的模块路径进行相对输出
protoc --proto_path="$ProtoDir" `
       --go_out="$RootDir" `
       --go_opt=module=go-llm-demo `
       "$ProtoFile"

if ($LASTEXITCODE -eq 0) {
    Write-Host "成功：代码已生成至 api/proto/chat.pb.go" -ForegroundColor Green
    Write-Host "提示：现在组员可以单独在测试代码中引入 'go-llm-demo/api/proto' 进行开发。" -ForegroundColor Gray
    Write-Host "注意：请勿手动修改生成的 .pb.go 文件！" -ForegroundColor Yellow
} else {
    Write-Host "失败：生成过程中出现错误，请检查 .proto 文件语法或插件是否安装。" -ForegroundColor Red
}
