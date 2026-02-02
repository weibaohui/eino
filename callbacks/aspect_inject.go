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
	"github.com/cloudwego/eino/internal/callbacks"
	"github.com/cloudwego/eino/schema"
)

// OnStart Fast inject callback input / output aspect for component developer
// e.g.
//
//	func (t *testChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (resp *schema.Message, err error) {
//		defer func() {
//			if err != nil {
//				callbacks.OnEnd(ctx, err)
//			}
//		}()
//
//		ctx = callbacks.OnStart(ctx, &model.CallbackInput{
//			Messages: input,
//			Tools:    nil,
//			Extra:    nil,
//		})
//
//		// do smt
//
//		ctx = callbacks.OnEnd(ctx, &model.CallbackOutput{
//			Message: resp,
//			Extra:   nil,
//		})
//
//		return resp, nil
//	}
//

// OnStart invokes the OnStart logic for the particular context, ensuring that all registered
// handlers are executed in reverse order (compared to add order) when a process begins.
//
// OnStart 为特定上下文调用 OnStart 逻辑，确保所有注册的处理程序在流程开始时按相反顺序（与添加顺序相比）执行。
func OnStart[T any](ctx context.Context, input T) context.Context {
	ctx, _ = callbacks.On(ctx, input, callbacks.OnStartHandle[T], TimingOnStart, true)

	return ctx
}

// OnEnd invokes the OnEnd logic of the particular context, allowing for proper cleanup
// and finalization when a process ends.
// handlers are executed in normal order (compared to add order).
//
// OnEnd 调用特定上下文的 OnEnd 逻辑，允许在流程结束时进行适当的清理和终结。
// 处理程序按正常顺序（与添加顺序相比）执行。
func OnEnd[T any](ctx context.Context, output T) context.Context {
	ctx, _ = callbacks.On(ctx, output, callbacks.OnEndHandle[T], TimingOnEnd, false)

	return ctx
}

// OnStartWithStreamInput invokes the OnStartWithStreamInput logic of the particular context, ensuring that
// every input stream should be closed properly in handler.
// handlers are executed in reverse order (compared to add order).
//
// OnStartWithStreamInput 调用特定上下文的 OnStartWithStreamInput 逻辑，确保每个输入流在处理程序中都应正确关闭。
// 处理程序按相反顺序（与添加顺序相比）执行。
func OnStartWithStreamInput[T any](ctx context.Context, input *schema.StreamReader[T]) (
	nextCtx context.Context, newStreamReader *schema.StreamReader[T]) {

	return callbacks.On(ctx, input, callbacks.OnStartWithStreamInputHandle[T], TimingOnStartWithStreamInput, true)
}

// OnEndWithStreamOutput invokes the OnEndWithStreamOutput logic of the particular, ensuring that
// every input stream should be closed properly in handler.
// handlers are executed in normal order (compared to add order).
//
// OnEndWithStreamOutput 调用特定的 OnEndWithStreamOutput 逻辑，确保每个输入流在处理程序中都应正确关闭。
// 处理程序按正常顺序（与添加顺序相比）执行。
func OnEndWithStreamOutput[T any](ctx context.Context, output *schema.StreamReader[T]) (
	nextCtx context.Context, newStreamReader *schema.StreamReader[T]) {

	return callbacks.On(ctx, output, callbacks.OnEndWithStreamOutputHandle[T], TimingOnEndWithStreamOutput, false)
}

// OnError invokes the OnError logic of the particular, notice that error in stream will not represent here.
// handlers are executed in normal order (compared to add order).
//
// OnError 调用特定的 OnError 逻辑，注意流中的错误不会在这里表示。
// 处理程序按正常顺序（与添加顺序相比）执行。
func OnError(ctx context.Context, err error) context.Context {
	ctx, _ = callbacks.On(ctx, err, callbacks.OnErrorHandle, TimingOnError, false)

	return ctx
}

// EnsureRunInfo ensures the RunInfo in context matches the given type and component.
// If the current callback manager doesn't match or doesn't exist, it creates a new one while preserving existing handlers.
// Will initialize Global callback handlers if none exist in the ctx before.
//
// EnsureRunInfo 确保上下文中的 RunInfo 与给定的类型和组件匹配。
// 如果当前的 callback manager 不匹配或不存在，它会在保留现有处理程序的同时创建一个新的。
// 如果之前上下文中不存在 Global callback handlers，将初始化它们。
func EnsureRunInfo(ctx context.Context, typ string, comp components.Component) context.Context {
	return callbacks.EnsureRunInfo(ctx, typ, comp)
}

// ReuseHandlers initializes a new context with the provided RunInfo, while using the same handlers already exist.
// Will initialize Global callback handlers if none exist in the ctx before.
//
// ReuseHandlers 使用提供的 RunInfo 初始化新上下文，同时使用已存在的相同处理程序。
// 如果之前上下文中不存在 Global callback handlers，将初始化它们。
func ReuseHandlers(ctx context.Context, info *RunInfo) context.Context {
	return callbacks.ReuseHandlers(ctx, info)
}

// InitCallbacks initializes a new context with the provided RunInfo and handlers.
// Any previously set RunInfo and Handlers for this ctx will be overwritten.
//
// InitCallbacks 使用提供的 RunInfo 和处理程序初始化新上下文。
// 该上下文之前设置的任何 RunInfo 和 Handlers 都将被覆盖。
func InitCallbacks(ctx context.Context, info *RunInfo, handlers ...Handler) context.Context {
	return callbacks.InitCallbacks(ctx, info, handlers...)
}
