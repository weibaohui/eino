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

package tool

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// BaseTool get tool info for ChatModel intent recognition.
//
// BaseTool 获取 ChatModel 意图识别的工具信息。
type BaseTool interface {
	Info(ctx context.Context) (*schema.ToolInfo, error)
}

// InvokableTool the tool for ChatModel intent recognition and ToolsNode execution.
//
// InvokableTool 是用于 ChatModel 意图识别和 ToolsNode 执行的工具。
type InvokableTool interface {
	BaseTool

	// InvokableRun call function with arguments in JSON format
	// InvokableRun 使用 JSON 格式的参数调用函数
	InvokableRun(ctx context.Context, argumentsInJSON string, opts ...Option) (string, error)
}

// StreamableTool the stream tool for ChatModel intent recognition and ToolsNode execution.
//
// StreamableTool 是用于 ChatModel 意图识别和 ToolsNode 执行的流式工具。
type StreamableTool interface {
	BaseTool

	// StreamableRun call function with arguments in JSON format and return stream reader
	// StreamableRun 使用 JSON 格式的参数调用函数并返回流式读取器
	StreamableRun(ctx context.Context, argumentsInJSON string, opts ...Option) (*schema.StreamReader[string], error)
}
