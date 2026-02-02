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
	"github.com/cloudwego/eino/internal/callbacks"
)

// RunInfo contains information about the running component.
//
// RunInfo 包含有关正在运行的组件的信息。
type RunInfo = callbacks.RunInfo

// CallbackInput is the input of the callback.
// the type of input is defined by the component.
// using type Assert or convert func to convert the input to the right type you want.
// e.g.
//
//		CallbackInput in components/model/interface.go is:
//		type CallbackInput struct {
//			Messages []*schema.Message
//			Config   *Config
//			Extra map[string]any
//		}
//
//	 and provide a func of model.ConvCallbackInput() to convert CallbackInput to *model.CallbackInput
//	 in callback handler, you can use the following code to get the input:
//
//		modelCallbackInput := model.ConvCallbackInput(in)
//		if modelCallbackInput == nil {
//			// is not a model callback input, just ignore it
//			return
//		}
//
// CallbackInput 是回调的输入。
// 输入的类型由组件定义。
// 使用类型断言或转换函数将输入转换为所需的正确类型。
type CallbackInput = callbacks.CallbackInput

// CallbackOutput is the unified callback output alias used by Eino.
//
// CallbackOutput 是 Eino 使用的统一回调输出别名。
type CallbackOutput = callbacks.CallbackOutput

// Handler is the unified callback handler alias used by Eino.
//
// Handler 是 Eino 使用的统一回调处理程序别名。
type Handler = callbacks.Handler

// InitCallbackHandlers sets the global callback handlers.
// It should be called BEFORE any callback handler by user.
// It's useful when you want to inject some basic callbacks to all nodes.
// Deprecated: Use AppendGlobalHandlers instead.
//
// InitCallbackHandlers 设置全局回调处理程序。
// 用户应在任何回调处理程序之前调用它。
// 当你想要向所有节点注入一些基本回调时，这很有用。
// 已弃用：请改用 AppendGlobalHandlers。
func InitCallbackHandlers(handlers []Handler) {
	callbacks.GlobalHandlers = handlers
}

// AppendGlobalHandlers appends the given handlers to the global callback handlers.
// This is the preferred way to add global callback handlers as it preserves existing handlers.
// The global callback handlers will be executed for all nodes BEFORE user-specific handlers in CallOption.
// Note: This function is not thread-safe and should only be called during process initialization.
//
// AppendGlobalHandlers 将给定的处理程序追加到全局回调处理程序。
// 这是添加全局回调处理程序的首选方式，因为它会保留现有的处理程序。
// 全局回调处理程序将在 CallOption 中的用户特定处理程序之前对所有节点执行。
// 注意：此函数不是线程安全的，只能在进程初始化期间调用。
func AppendGlobalHandlers(handlers ...Handler) {
	callbacks.GlobalHandlers = append(callbacks.GlobalHandlers, handlers...)
}

// CallbackTiming enumerates all the timing of callback aspects.
//
// CallbackTiming 枚举了回调切面的所有时机。
type CallbackTiming = callbacks.CallbackTiming

// CallbackTiming values enumerate the lifecycle moments when handlers run.
//
// CallbackTiming 值枚举了处理程序运行的生命周期时刻。
const (
	TimingOnStart CallbackTiming = iota
	TimingOnEnd
	TimingOnError
	TimingOnStartWithStreamInput
	TimingOnEndWithStreamOutput
)

// TimingChecker checks if the handler is needed for the given callback aspect timing.
// It's recommended for callback handlers to implement this interface, but not mandatory.
// If a callback handler is created by using callbacks.HandlerHelper or handlerBuilder, then this interface is automatically implemented.
// Eino's callback mechanism will try to use this interface to determine whether any handlers are needed for the given timing.
// Also, the callback handler that is not needed for that timing will be skipped.
//
// TimingChecker 检查给定的回调切面时机是否需要处理程序。
// 建议回调处理程序实现此接口，但不是强制性的。
// 如果回调处理程序是使用 callbacks.HandlerHelper 或 handlerBuilder 创建的，则会自动实现此接口。
// Eino 的回调机制将尝试使用此接口来确定给定给定时机是否需要任何处理程序。
// 此外，将跳过该时机不需要的回调处理程序。
type TimingChecker = callbacks.TimingChecker
