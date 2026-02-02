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

package reduction

import (
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/adk/filesystem"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// Backend defines the interface provided by the user to implement file storage.
// It is used to save the content of large tool results to a persistent storage.
// Backend 定义了用户实现文件存储的接口。
// 它用于将大型工具结果的内容保存到持久存储中。
type Backend interface {
	Write(context.Context, *filesystem.WriteRequest) error
}

// ToolResultConfig configures the tool result reduction middleware.
// ToolResultConfig 配置工具结果缩减中间件。
type ToolResultConfig struct {
	// ClearingTokenThreshold is the threshold for the total token count of all tool results.
	// When the sum of all tool result tokens exceeds this threshold, old tool results
	// (outside the KeepRecentTokens range) will be replaced with a placeholder.
	// Token estimation uses a simple heuristic: character count / 4.
	// optional, 20000 by default
	// ClearingTokenThreshold 是所有工具结果总 token 数量的阈值。
	// 当所有工具结果 token 的总和超过此阈值时，旧的工具结果（在 KeepRecentTokens 范围之外）将被替换为占位符。
	// Token 估算使用简单的启发式方法：字符数 / 4。
	// 可选，默认为 20000。
	ClearingTokenThreshold int

	// KeepRecentTokens is the token budget for recent messages to keep intact.
	// Messages within this token budget from the end will not have their tool results cleared,
	// even if the total tool result tokens exceed the threshold.
	// optional, 40000 by default
	// KeepRecentTokens 是要保持完整的最近消息的 token 预算。
	// 即使总工具结果 token 超过阈值，位于末尾此 token 预算内的消息的工具结果也不会被清理。
	// 可选，默认为 40000。
	KeepRecentTokens int

	// ClearToolResultPlaceholder is the text to replace old tool results with.
	// optional, "[Old tool result content cleared]" by default
	// ClearToolResultPlaceholder 用于替换旧工具结果的文本。
	// 可选，默认为 "[Old tool result content cleared]"。
	ClearToolResultPlaceholder string

	// TokenCounter is a custom function to estimate token count for a message.
	// optional, uses the default counter (character count / 4) if nil
	// TokenCounter 是用于估算消息 token 数量的自定义函数。
	// 可选，如果为 nil，使用默认计数器（字符数 / 4）。
	TokenCounter func(msg *schema.Message) int

	// ExcludeTools is a list of tool names whose results should never be cleared.
	// optional
	// ExcludeTools 是结果永远不应被清理的工具名称列表。
	// 可选。
	ExcludeTools []string

	// Backend is the storage backend for offloaded tool results.
	// required
	// Backend 是卸载工具结果的存储后端。
	// 必填。
	Backend Backend

	// OffloadingTokenLimit is the token threshold for a single tool result to trigger offloading.
	// When a single tool result exceeds OffloadingTokenLimit * 4 characters, it will be
	// offloaded to the filesystem.
	// optional, 20000 by default
	// OffloadingTokenLimit 是触发卸载的单个工具结果的 token 阈值。
	// 当单个工具结果超过 OffloadingTokenLimit * 4 个字符时，它将被卸载到文件系统。
	// 可选，默认为 20000。
	OffloadingTokenLimit int

	// ReadFileToolName is the name of the tool that LLM should use to read offloaded content.
	// This name will be included in the summary message sent to the LLM.
	// optional, "read_file" by default
	//
	// NOTE: If you are using the filesystem middleware, the read_file tool name
	// is exactly "read_file", which matches the default value.
	// ReadFileToolName 是 LLM 应使用来读取卸载内容的工具名称。
	// 此名称将包含在发送给 LLM 的摘要消息中。
	// 可选，默认为 "read_file"。
	//
	// 注意：如果你正在使用文件系统中间件，read_file 工具名称正是 "read_file"，与默认值匹配。
	ReadFileToolName string

	// PathGenerator generates the write path for offloaded results.
	// optional, "/large_tool_result/{ToolCallID}" by default
	// PathGenerator 生成卸载结果的写入路径。
	// 可选，默认为 "/large_tool_result/{ToolCallID}"。
	PathGenerator func(ctx context.Context, input *compose.ToolInput) (string, error)
}

// NewToolResultMiddleware creates a tool result reduction middleware.
// This middleware combines two strategies to manage tool result tokens:
//
//  1. Clearing: Replaces old tool results with a placeholder when the total
//     tool result tokens exceed the threshold, while protecting recent messages.
//
//  2. Offloading: Writes large individual tool results to the filesystem and
//     returns a summary message guiding the LLM to read the full content.
//
// NOTE: If you are using the filesystem middleware (github.com/cloudwego/eino/adk/middlewares/filesystem),
// this functionality is already included by default. Set Config.WithoutLargeToolResultOffloading = true
// in the filesystem middleware if you want to use this middleware separately instead.
//
// NOTE: This middleware only handles offloading results to the filesystem.
// You MUST also provide a read_file tool to your agent, otherwise the agent
// will not be able to read the offloaded content. You can either:
//   - Use the filesystem middleware (github.com/cloudwego/eino/adk/middlewares/filesystem)
//     which provides the read_file tool automatically, OR
//   - Implement your own read_file tool that reads from the same Backend
//
// NewToolResultMiddleware 创建一个工具结果缩减中间件。
// 该中间件结合了两种策略来管理工具结果 token：
//
//  1. 清理：当总工具结果 token 超过阈值时，用占位符替换旧的工具结果，同时保护最近的消息。
//
//  2. 卸载：将大的单个工具结果写入文件系统，并返回引导 LLM 阅读完整内容的摘要消息。
//
// 注意：如果你正在使用文件系统中间件 (github.com/cloudwego/eino/adk/middlewares/filesystem)，
// 此功能默认已包含。如果你想单独使用此中间件，请在文件系统中间件中设置 Config.WithoutLargeToolResultOffloading = true。
//
// 注意：此中间件仅处理将结果卸载到文件系统。
// 你必须同时为你的 agent 提供 read_file 工具，否则 agent 将无法读取卸载的内容。你可以：
//   - 使用文件系统中间件 (github.com/cloudwego/eino/adk/middlewares/filesystem)，它自动提供 read_file 工具，或者
//   - 实现你自己的 read_file 工具，从相同的 Backend 读取。
func NewToolResultMiddleware(ctx context.Context, cfg *ToolResultConfig) (adk.AgentMiddleware, error) {
	bc := newClearToolResult(ctx, &ClearToolResultConfig{
		ToolResultTokenThreshold:   cfg.ClearingTokenThreshold,
		KeepRecentTokens:           cfg.KeepRecentTokens,
		ClearToolResultPlaceholder: cfg.ClearToolResultPlaceholder,
		TokenCounter:               cfg.TokenCounter,
		ExcludeTools:               cfg.ExcludeTools,
	})
	tm := newToolResultOffloading(ctx, &toolResultOffloadingConfig{
		Backend:          cfg.Backend,
		ReadFileToolName: cfg.ReadFileToolName,
		TokenLimit:       cfg.OffloadingTokenLimit,
		PathGenerator:    cfg.PathGenerator,
	})
	return adk.AgentMiddleware{
		BeforeChatModel: bc,
		WrapToolCall:    tm,
	}, nil
}
