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

	"github.com/cloudwego/eino/schema"
)

// HandlerBuilder constructs a Handler by chaining callback functions
// for start, end, error, and streaming aspects.
//
// HandlerBuilder 通过链接 start、end、error 和流式切面的回调函数来构造 Handler。
type HandlerBuilder struct {
	onStartFn                func(ctx context.Context, info *RunInfo, input CallbackInput) context.Context
	onEndFn                  func(ctx context.Context, info *RunInfo, output CallbackOutput) context.Context
	onErrorFn                func(ctx context.Context, info *RunInfo, err error) context.Context
	onStartWithStreamInputFn func(ctx context.Context, info *RunInfo, input *schema.StreamReader[CallbackInput]) context.Context
	onEndWithStreamOutputFn  func(ctx context.Context, info *RunInfo, output *schema.StreamReader[CallbackOutput]) context.Context
}

type handlerImpl struct {
	HandlerBuilder
}

func (hb *handlerImpl) OnStart(ctx context.Context, info *RunInfo, input CallbackInput) context.Context {
	return hb.onStartFn(ctx, info, input)
}

func (hb *handlerImpl) OnEnd(ctx context.Context, info *RunInfo, output CallbackOutput) context.Context {
	return hb.onEndFn(ctx, info, output)
}

func (hb *handlerImpl) OnError(ctx context.Context, info *RunInfo, err error) context.Context {
	return hb.onErrorFn(ctx, info, err)
}

func (hb *handlerImpl) OnStartWithStreamInput(ctx context.Context, info *RunInfo,
	input *schema.StreamReader[CallbackInput]) context.Context {

	return hb.onStartWithStreamInputFn(ctx, info, input)
}

func (hb *handlerImpl) OnEndWithStreamOutput(ctx context.Context, info *RunInfo,
	output *schema.StreamReader[CallbackOutput]) context.Context {

	return hb.onEndWithStreamOutputFn(ctx, info, output)
}

func (hb *handlerImpl) Needed(_ context.Context, _ *RunInfo, timing CallbackTiming) bool {
	switch timing {
	case TimingOnStart:
		return hb.onStartFn != nil
	case TimingOnEnd:
		return hb.onEndFn != nil
	case TimingOnError:
		return hb.onErrorFn != nil
	case TimingOnStartWithStreamInput:
		return hb.onStartWithStreamInputFn != nil
	case TimingOnEndWithStreamOutput:
		return hb.onEndWithStreamOutputFn != nil
	default:
		return false
	}
}

// NewHandlerBuilder creates and returns a new HandlerBuilder instance.
// HandlerBuilder is used to construct a Handler with custom callback functions
//
// NewHandlerBuilder 创建并返回一个新的 HandlerBuilder 实例。
// HandlerBuilder 用于使用自定义回调函数构造 Handler。
func NewHandlerBuilder() *HandlerBuilder {
	return &HandlerBuilder{}
}

// OnStartFn sets the handler for the start timing.
//
// OnStartFn 设置 start 时机的处理程序。
func (hb *HandlerBuilder) OnStartFn(
	fn func(ctx context.Context, info *RunInfo, input CallbackInput) context.Context) *HandlerBuilder {

	hb.onStartFn = fn
	return hb
}

// OnEndFn sets the handler for the end timing.
//
// OnEndFn 设置 end 时机的处理程序。
func (hb *HandlerBuilder) OnEndFn(
	fn func(ctx context.Context, info *RunInfo, output CallbackOutput) context.Context) *HandlerBuilder {

	hb.onEndFn = fn
	return hb
}

// OnErrorFn sets the handler for the error timing.
//
// OnErrorFn 设置 error 时机的处理程序。
func (hb *HandlerBuilder) OnErrorFn(
	fn func(ctx context.Context, info *RunInfo, err error) context.Context) *HandlerBuilder {

	hb.onErrorFn = fn
	return hb
}

// OnStartWithStreamInputFn sets the callback function to be called.
//
// OnStartWithStreamInputFn 设置要调用的回调函数。
func (hb *HandlerBuilder) OnStartWithStreamInputFn(
	fn func(ctx context.Context, info *RunInfo, input *schema.StreamReader[CallbackInput]) context.Context) *HandlerBuilder {

	hb.onStartWithStreamInputFn = fn
	return hb
}

// OnEndWithStreamOutputFn sets the callback function to be called.
//
// OnEndWithStreamOutputFn 设置要调用的回调函数。
func (hb *HandlerBuilder) OnEndWithStreamOutputFn(
	fn func(ctx context.Context, info *RunInfo, output *schema.StreamReader[CallbackOutput]) context.Context) *HandlerBuilder {

	hb.onEndWithStreamOutputFn = fn
	return hb
}

// Build returns a Handler with the functions set in the builder.
//
// Build 返回一个包含构建器中设置的函数的 Handler。
func (hb *HandlerBuilder) Build() Handler {
	return &handlerImpl{*hb}
}
