/*
 * Copyright 2024 CloudWeGo Authors
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

// Package react provides helpers to build callback handlers for React agents.
package react

import (
	"context"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/schema"
)

// AgentCallback is the callback for the agent.
// AgentCallback 是 Agent 的回调接口。
type AgentCallback interface {
	// OnAgentStart is called when the agent starts.
	// OnAgentStart 在 Agent 启动时被调用。
	OnAgentStart(ctx context.Context, input *schema.Message) context.Context
	// OnAgentEnd is called when the agent ends.
	// OnAgentEnd 在 Agent 结束时被调用。
	OnAgentEnd(ctx context.Context, output *schema.Message) context.Context
}

// BuildAgentCallback builds the callback handler for the agent.
// BuildAgentCallback 构建 Agent 的回调处理器。
func BuildAgentCallback(handlers ...AgentCallback) callbacks.Handler {
	return callbacks.NewHandlerBuilder().
		OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
			// 检查是否有关联的 handlers，如果没有则直接返回
			if len(handlers) == 0 {
				return ctx
			}

			// 获取输入的 Message
			msg := input.(map[string]any)["input_message"].(*schema.Message)

			// 遍历所有 handlers，调用 OnAgentStart
			for _, h := range handlers {
				ctx = h.OnAgentStart(ctx, msg)
			}
			return ctx
		}).
		OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
			// 检查是否有关联的 handlers，如果没有则直接返回
			if len(handlers) == 0 {
				return ctx
			}

			// 获取输出的 Message
			msg := output.(map[string]any)["output_message"].(*schema.Message)

			// 遍历所有 handlers，调用 OnAgentEnd
			for _, h := range handlers {
				ctx = h.OnAgentEnd(ctx, msg)
			}
			return ctx
		}).Build()
}
