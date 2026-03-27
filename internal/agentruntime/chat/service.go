package chat

import (
	"context"
	"fmt"
	"strings"

	"neo-code/internal/agentruntime/memory"
	"neo-code/internal/agentruntime/session"
	"neo-code/internal/agentruntime/todo"
	toolprotocol "neo-code/internal/tool/protocol"
	toolregistry "neo-code/internal/tool/registry"
)

type chatServiceImpl struct {
	memorySvc    memory.MemoryService
	workingSvc   session.WorkingMemoryService
	todoSvc      todo.TodoService
	promptSvc    PromptProvider
	chatProvider ChatProvider
}

func NewChatService(
	memorySvc memory.MemoryService,
	workingSvc session.WorkingMemoryService,
	todoSvc todo.TodoService,
	promptSvc PromptProvider,
	chatProvider ChatProvider,
) ChatGateway {
	return &chatServiceImpl{
		memorySvc:    memorySvc,
		workingSvc:   workingSvc,
		todoSvc:      todoSvc,
		promptSvc:    promptSvc,
		chatProvider: chatProvider,
	}
}

func (s *chatServiceImpl) Send(ctx context.Context, req *ChatRequest) (<-chan string, error) {
	messages := append([]Message{}, req.Messages...)

	rolePrompt := ""
	if s.promptSvc != nil {
		var err error
		rolePrompt, err = s.promptSvc.GetActivePrompt(ctx)
		if err != nil {
			fmt.Printf("load active prompt failed: %v\n", err)
		} else if rolePrompt != "" && !hasSystemMessage(messages) {
			messages = append([]Message{{Role: "system", Content: rolePrompt}}, messages...)
		}
	}

	userInput := s.latestUserInput(messages)
	workingContext := ""
	var err error
	if s.workingSvc != nil {
		workingContext, err = s.workingSvc.BuildContext(ctx, toWorkingMemoryMessages(messages))
		if err != nil {
			return nil, err
		}
	}

	todoContext := ""
	if s.todoSvc != nil {
		todos, _ := s.todoSvc.ListTodos(ctx)
		todoContext = buildTodoContext(todos)
	}

	toolContext := toolprotocol.RenderInstructionBlock(toolregistry.GlobalRegistry.ListDefinitions())
	blocks := []string{toolContext, workingContext, todoContext}
	if userInput != "" && s.memorySvc != nil {
		memoryContext, ctxErr := s.memorySvc.BuildContext(ctx, userInput)
		if ctxErr != nil {
			return nil, ctxErr
		}
		blocks = append(blocks, memoryContext)
	}
	if combinedContext := joinContextBlocks(blocks...); combinedContext != "" {
		messages = injectSystemContext(messages, rolePrompt, combinedContext)
	}

	out, err := s.chatProvider.Chat(ctx, messages)
	if err != nil {
		return nil, err
	}

	resultChan := make(chan string)
	go func() {
		defer close(resultChan)

		var replyBuilder strings.Builder
		for chunk := range out {
			replyBuilder.WriteString(chunk)
			resultChan <- chunk
		}

		if userInput == "" || replyBuilder.Len() == 0 {
			return
		}

		if s.workingSvc != nil {
			updatedMessages := append([]Message{}, req.Messages...)
			updatedMessages = append(updatedMessages, Message{Role: "assistant", Content: replyBuilder.String()})
			if err := s.workingSvc.Refresh(context.Background(), toWorkingMemoryMessages(updatedMessages)); err != nil {
				fmt.Printf("refresh working memory failed: %v\n", err)
			}
		}
		if s.memorySvc != nil {
			if err := s.memorySvc.Save(context.Background(), userInput, replyBuilder.String()); err != nil {
				fmt.Printf("save memory failed: %v\n", err)
			}
		}
	}()

	return resultChan, nil
}

func hasSystemMessage(messages []Message) bool {
	for _, msg := range messages {
		if msg.Role == "system" {
			return true
		}
	}
	return false
}

func (s *chatServiceImpl) latestUserInput(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			return strings.TrimSpace(messages[i].Content)
		}
	}
	return ""
}

func buildTodoContext(todos []todo.Todo) string {
	if len(todos) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("[TODO_LIST]\n")
	for _, todo := range todos {
		sb.WriteString(fmt.Sprintf("- %s: %s (status: %s, priority: %s)\n", todo.ID, todo.Content, todo.Status, todo.Priority))
	}
	return sb.String()
}

func injectSystemContext(messages []Message, rolePrompt, combinedContext string) []Message {
	if rolePrompt != "" && len(messages) > 0 && messages[0].Role == "system" {
		messages[0].Content = rolePrompt + "\n\n" + combinedContext
		return messages
	}
	return append([]Message{{Role: "system", Content: combinedContext}}, messages...)
}

func joinContextBlocks(blocks ...string) string {
	filtered := make([]string, 0, len(blocks))
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		filtered = append(filtered, block)
	}
	return strings.Join(filtered, "\n\n")
}

func toWorkingMemoryMessages(messages []Message) []session.Message {
	converted := make([]session.Message, 0, len(messages))
	for _, msg := range messages {
		converted = append(converted, session.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}
	return converted
}
