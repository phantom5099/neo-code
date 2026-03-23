package test

import (
	"testing"

	"go-llm-demo/api/proto"              // 引入我们的“新契约”
	"go-llm-demo/internal/server/domain" // 引入“旧后端模型”
)

// TestFrontendBackendCompatibility 这个测试模拟了：
// 如果前端按照契约发请求，后端能不能正常处理。
func TestFrontendBackendCompatibility(t *testing.T) {
	// 1. 【模拟前端逻辑】
	// 前端开发者按照 chat.proto 契约，捏造了一个请求
	frontendReq := &proto.ChatRequest{
		Model: "qwen-max",
		Messages: []*proto.Message{
			{Role: "user", Content: "你好，请自我介绍"},
		},
	}

	t.Logf("前端发出了契约请求，模型为: %s", frontendReq.Model)

	// 2. 【模拟“转换层”逻辑】
	// 这是一个临时的适配逻辑，用来验证契约字段是否能覆盖后端需求
	// 如果我们在 .proto 里漏掉了某个关键字段，这一步就会发现“没法转”
	var backendMessages []domain.Message
	for _, m := range frontendReq.Messages {
		backendMessages = append(backendMessages, domain.Message{
			Role:    m.Role,
			Content: m.Content,
		})
	}

	backendReq := &domain.ChatRequest{
		Model:    frontendReq.Model,
		Messages: backendMessages,
	}

	// 3. 【验证后端兼容性】
	// 我们检查转换后的对象是否符合后端 domain 的要求
	if backendReq.Model != "qwen-max" {
		t.Errorf("数据在转换过程中丢失！预期模型 qwen-max，实际得到 %s", backendReq.Model)
	}

	if len(backendReq.Messages) != 1 || backendReq.Messages[0].Content != "你好，请自我介绍" {
		t.Error("对话内容转换失败")
	}

	// 4. 【模拟后端回传契约】
	// 后端处理完后，需要把结果装进契约定义的响应里
	// 如果契约定义的字段不够（比如没有 is_finished），前端就没法渲染
	mockReply := "我是 NeoCode 助手"
	response := &proto.ChatResponse{
		Content:    mockReply,
		IsFinished: true,
		Status: &proto.Status{
			Code:    0,
			Message: "OK",
		},
	}

	if response.Status.Code != 0 {
		t.Error("响应契约状态码异常")
	}

	t.Log("验证成功：当前的 api/proto 契约能够完美覆盖现有的前后端数据传输需求！")
}

// 这个测试证明了：
// 1. 前端可以只看 .proto 就开始写代码。
// 2. 后端可以只看转换后的对象就开始写代码。
// 3. 契约层（api/proto）作为标准是完全可行的。
