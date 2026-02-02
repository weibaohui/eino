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

package model

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// BaseChatModel defines the basic interface for chat models.
// It provides methods for generating complete outputs and streaming outputs.
// This interface serves as the foundation for all chat model implementations.
//
// BaseChatModel 定义了聊天模型的基本接口。
// 它提供了生成完整输出和流式输出的方法。
// 该接口是所有聊天模型实现的基础。
//
//go:generate  mockgen -destination ../../internal/mock/components/model/ChatModel_mock.go --package model -source interface.go
type BaseChatModel interface {
	Generate(ctx context.Context, input []*schema.Message, opts ...Option) (*schema.Message, error)
	Stream(ctx context.Context, input []*schema.Message, opts ...Option) (
		*schema.StreamReader[*schema.Message], error)
}

// Deprecated: Please use ToolCallingChatModel interface instead, which provides a safer way to bind tools
// without the concurrency issues and tool overwriting problems that may arise from the BindTools method.
//
// Deprecated: 请使用 ToolCallingChatModel 接口代替，它提供了一种更安全的方式来绑定工具，
// 避免了 BindTools 方法可能出现的并发问题和工具覆盖问题。
type ChatModel interface {
	BaseChatModel

	// BindTools bind tools to the model.
	// BindTools before requesting ChatModel generally.
	// notice the non-atomic problem of BindTools and Generate.
	//
	// BindTools 将工具绑定到模型。
	// 通常在请求 ChatModel 之前绑定工具。
	// 注意 BindTools 和 Generate 的非原子性问题。
	BindTools(tools []*schema.ToolInfo) error
}

// ToolCallingChatModel extends BaseChatModel with tool calling capabilities.
// It provides a WithTools method that returns a new instance with
// the specified tools bound, avoiding state mutation and concurrency issues.
//
// ToolCallingChatModel 扩展了 BaseChatModel，具有工具调用功能。
// 它提供了一个 WithTools 方法，返回绑定了指定工具的新实例，
// 避免了状态突变和并发问题。
type ToolCallingChatModel interface {
	BaseChatModel

	// WithTools returns a new ToolCallingChatModel instance with the specified tools bound.
	// This method does not modify the current instance, making it safer for concurrent use.
	//
	// WithTools 返回一个绑定了指定工具的新 ToolCallingChatModel 实例。
	// 此方法不会修改当前实例，使其在并发使用时更安全。
	WithTools(tools []*schema.ToolInfo) (ToolCallingChatModel, error)
}
