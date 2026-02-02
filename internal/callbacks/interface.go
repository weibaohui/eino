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

package callbacks

import (
	"context"

	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/schema"
)

// RunInfo 运行时信息，包含节点名称、组件类型和实例
type RunInfo struct {
	// Name is the graph node name for display purposes, not unique.
	// Passed from compose.WithNodeName().
	// Name 节点名称（用于展示，不唯一）
	Name string
	// Type 组件类型（如 "node", "chain" 等）
	Type string
	// Component 组件实例
	Component components.Component
}

// CallbackInput 回调输入类型（泛型）
type CallbackInput any

// CallbackOutput 回调输出类型（泛型）
type CallbackOutput any

// Handler 回调处理器接口
// - 用户：希望在组件执行生命周期插入监控/日志/指标的使用者
// - 使用：实现 Handler 并通过 InitCallbacks/AppendHandlers 注入上下文
// - 事件：开始/结束（支持常规与流式），以及错误事件
type Handler interface {
	OnStart(ctx context.Context, info *RunInfo, input CallbackInput) context.Context
	OnEnd(ctx context.Context, info *RunInfo, output CallbackOutput) context.Context

	OnError(ctx context.Context, info *RunInfo, err error) context.Context

	OnStartWithStreamInput(ctx context.Context, info *RunInfo,
		input *schema.StreamReader[CallbackInput]) context.Context
	OnEndWithStreamOutput(ctx context.Context, info *RunInfo,
		output *schema.StreamReader[CallbackOutput]) context.Context
}

// CallbackTiming 回调触发时机枚举
type CallbackTiming uint8

// TimingChecker 用于按时机筛选是否需要触发当前 Handler
type TimingChecker interface {
	Needed(ctx context.Context, info *RunInfo, timing CallbackTiming) bool
}
