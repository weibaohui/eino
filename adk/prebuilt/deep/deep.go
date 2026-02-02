/*
 * Copyright 2025 CloudWeGo Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package deep provides a prebuilt agent with deep task orchestration.
package deep

import (
	"context"
	"fmt"

	"github.com/bytedance/sonic"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

func init() {
	schema.RegisterName[TODO]("_eino_adk_prebuilt_deep_todo")
	schema.RegisterName[[]TODO]("_eino_adk_prebuilt_deep_todo_slice")
}

// Config 定义创建 DeepAgent 的配置。
type Config struct {
	// Name 是 Deep Agent 的标识符。
	Name string
	// Description 提供 Agent 用途的简要说明。
	Description string

	// ChatModel 是 DeepAgent 用于推理和执行任务的模型。
	ChatModel model.ToolCallingChatModel
	// Instruction 包含指导 Agent 行为的系统提示词。
	// 当为空时，将使用内置的默认系统提示词，其中包含通用助手行为准则、安全策略、编码风格指南和工具使用规范。
	Instruction string
	// SubAgents 是可以被该 Agent 调用的专用子 Agent。
	SubAgents []adk.Agent
	// ToolsConfig 提供 Agent 可调用的工具和工具调用配置。
	ToolsConfig adk.ToolsConfig
	// MaxIteration 限制 Agent 可以执行的最大推理迭代次数。
	MaxIteration int

	// WithoutWriteTodos 设置为 true 时禁用内置的 write_todos 工具。
	WithoutWriteTodos bool
	// WithoutGeneralSubAgent 设置为 true 时禁用通用的子 Agent。
	WithoutGeneralSubAgent bool
	// TaskToolDescriptionGenerator 允许自定义 task 工具的描述。
	// 如果提供，此函数将根据可用的子 Agent 生成工具描述。
	TaskToolDescriptionGenerator func(ctx context.Context, availableAgents []adk.Agent) (string, error)

	// Middlewares 是要添加到 Agent 的中间件列表。
	Middlewares []adk.AgentMiddleware

	// ModelRetryConfig 配置模型的重试策略。
	ModelRetryConfig *adk.ModelRetryConfig

	// OutputKey 将 Agent 的响应存储在 Session 中。
	// 可选。当设置时，通过 AddSessionValue(ctx, outputKey, msg.Content) 存储输出。
	OutputKey string
}

// New 创建一个新的 Deep Agent 实例。
// 该函数初始化内置工具，创建用于子 Agent 编排的 task 工具，
// 并返回一个配置好的 ChatModelAgent 准备执行。
func New(ctx context.Context, cfg *Config) (adk.ResumableAgent, error) {
	middlewares, err := buildBuiltinAgentMiddlewares(cfg.WithoutWriteTodos)
	if err != nil {
		return nil, err
	}

	instruction := cfg.Instruction
	if len(instruction) == 0 {
		instruction = baseAgentInstruction
	}

	if !cfg.WithoutGeneralSubAgent || len(cfg.SubAgents) > 0 {
		tt, err := newTaskToolMiddleware(
			ctx,
			cfg.TaskToolDescriptionGenerator,
			cfg.SubAgents,

			cfg.WithoutGeneralSubAgent,
			cfg.ChatModel,
			instruction,
			cfg.ToolsConfig,
			cfg.MaxIteration,
			append(middlewares, cfg.Middlewares...),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to new task tool: %w", err)
		}
		middlewares = append(middlewares, tt)
	}

	return adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          cfg.Name,
		Description:   cfg.Description,
		Instruction:   instruction,
		Model:         cfg.ChatModel,
		ToolsConfig:   cfg.ToolsConfig,
		MaxIterations: cfg.MaxIteration,
		Middlewares:   append(middlewares, cfg.Middlewares...),

		GenModelInput:    genModelInput,
		ModelRetryConfig: cfg.ModelRetryConfig,
		OutputKey:        cfg.OutputKey,
	})
}

// genModelInput 生成发送给模型的输入消息列表。
// 它将指令（Instruction）作为系统消息添加到消息列表的开头。
func genModelInput(ctx context.Context, instruction string, input *adk.AgentInput) ([]*schema.Message, error) {
	msgs := make([]*schema.Message, 0, len(input.Messages)+1)

	if instruction != "" {
		msgs = append(msgs, schema.SystemMessage(instruction))
	}

	msgs = append(msgs, input.Messages...)

	return msgs, nil
}

// buildBuiltinAgentMiddlewares 构建内置的 Agent 中间件。
// 为什么要做这个：根据配置初始化必要的中间件，如 write_todos 工具。
func buildBuiltinAgentMiddlewares(withoutWriteTodos bool) ([]adk.AgentMiddleware, error) {
	var ms []adk.AgentMiddleware
	if !withoutWriteTodos {
		t, err := newWriteTodos()
		if err != nil {
			return nil, err
		}
		ms = append(ms, t)
	}

	return ms, nil
}

// TODO 表示任务列表中的一个待办事项。
type TODO struct {
	// Content 是待办事项的具体内容描述。
	Content string `json:"content"`
	// ActiveForm 描述了当前事项的活动形式或上下文。
	ActiveForm string `json:"activeForm"`
	// Status 是事项的当前状态：pending(待处理), in_progress(进行中), completed(已完成)。
	Status string `json:"status" jsonschema:"enum=pending,enum=in_progress,enum=completed"`
}

// writeTodosArguments 定义 write_todos 工具的参数结构。
type writeTodosArguments struct {
	// Todos 是更新后的待办事项列表。
	Todos []TODO `json:"todos"`
}

// newWriteTodos 创建 write_todos 工具的中间件。
// 该工具允许 Agent 管理任务列表，记录进度并在 Session 中存储状态。
func newWriteTodos() (adk.AgentMiddleware, error) {
	t, err := utils.InferTool("write_todos", writeTodosToolDescription, func(ctx context.Context, input writeTodosArguments) (output string, err error) {
		adk.AddSessionValue(ctx, SessionKeyTodos, input.Todos)
		todos, err := sonic.MarshalString(input.Todos)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Updated todo list to %s", todos), nil
	})
	if err != nil {
		return adk.AgentMiddleware{}, err
	}

	return adk.AgentMiddleware{
		AdditionalInstruction: writeTodosPrompt,
		AdditionalTools:       []tool.BaseTool{t},
	}, nil
}
