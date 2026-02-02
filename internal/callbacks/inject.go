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
	"github.com/cloudwego/eino/internal/generic"
	"github.com/cloudwego/eino/schema"
)

// InitCallbacks 初始化回调上下文
// - 用户：希望为组件执行过程注入回调的使用者
// - 使用：ctx = InitCallbacks(ctx, &RunInfo{Type: "node", Component: comp}, handlers...)
// - 逻辑：若存在处理器则创建管理器；否则注入 nil 管理器
func InitCallbacks(ctx context.Context, info *RunInfo, handlers ...Handler) context.Context {
	mgr, ok := newManager(info, handlers...)
	if ok {
		return ctxWithManager(ctx, mgr)
	}

	return ctxWithManager(ctx, nil)
}

// EnsureRunInfo 确保回调上下文包含 RunInfo（若为空则填充）
// - 用户：在组件入口处确保上下文拥有正确的运行信息
func EnsureRunInfo(ctx context.Context, typ string, comp components.Component) context.Context {
	cbm, ok := managerFromCtx(ctx)
	if !ok {
		return InitCallbacks(ctx, &RunInfo{
			Type:      typ,
			Component: comp,
		})
	}
	if cbm.runInfo == nil {
		return ReuseHandlers(ctx, &RunInfo{
			Type:      typ,
			Component: comp,
		})
	}
	return ctx
}

// ReuseHandlers 复用已有的 handlers，并更新 RunInfo
func ReuseHandlers(ctx context.Context, info *RunInfo) context.Context {
	cbm, ok := managerFromCtx(ctx)
	if !ok {
		return InitCallbacks(ctx, info)
	}
	return ctxWithManager(ctx, cbm.withRunInfo(info))
}

// AppendHandlers 追加新的 handlers，并生成新的管理器
func AppendHandlers(ctx context.Context, info *RunInfo, handlers ...Handler) context.Context {
	cbm, ok := managerFromCtx(ctx)
	if !ok {
		return InitCallbacks(ctx, info, handlers...)
	}
	nh := make([]Handler, len(cbm.handlers)+len(handlers))
	copy(nh[:len(cbm.handlers)], cbm.handlers)
	copy(nh[len(cbm.handlers):], handlers)
	return InitCallbacks(ctx, info, nh...)
}

type Handle[T any] func(context.Context, T, *RunInfo, []Handler) (context.Context, T)

// On 统一的“按时机触发处理器”入口
// - start=true：从 RunInfo 中取信息并清空，随后存入上下文，适用于 OnStart
// - start=false：优先使用当前 RunInfo，否则从上下文取历史值，适用于 OnEnd/OnError
// - 逻辑：按 TimingChecker 过滤需要处理的 handlers，调用 handle 回调
func On[T any](ctx context.Context, inOut T, handle Handle[T], timing CallbackTiming, start bool) (context.Context, T) {
	mgr, ok := managerFromCtx(ctx)
	if !ok {
		return ctx, inOut
	}
	nMgr := *mgr

	var info *RunInfo
	if start {
		info = nMgr.runInfo
		nMgr.runInfo = nil
		ctx = context.WithValue(ctx, CtxRunInfoKey{}, info)
	} else {
		if nMgr.runInfo != nil {
			info = nMgr.runInfo
		} else {
			info, _ = ctx.Value(CtxRunInfoKey{}).(*RunInfo)
		}
	}

	hs := make([]Handler, 0, len(nMgr.handlers)+len(nMgr.globalHandlers))
	for _, handler := range append(nMgr.handlers, nMgr.globalHandlers...) {
		timingChecker, ok_ := handler.(TimingChecker)
		if !ok_ || timingChecker.Needed(ctx, info, timing) {
			hs = append(hs, handler)
		}
	}

	var out T
	ctx, out = handle(ctx, inOut, info, hs)
	return ctxWithManager(ctx, &nMgr), out
}

// OnStartHandle 触发非流式 OnStart 处理器（逆序调用）
func OnStartHandle[T any](ctx context.Context, input T,
	runInfo *RunInfo, handlers []Handler) (context.Context, T) {

	for i := len(handlers) - 1; i >= 0; i-- {
		ctx = handlers[i].OnStart(ctx, runInfo, input)
	}

	return ctx, input
}

// OnEndHandle 触发非流式 OnEnd 处理器（顺序调用）
func OnEndHandle[T any](ctx context.Context, output T,
	runInfo *RunInfo, handlers []Handler) (context.Context, T) {

	for _, handler := range handlers {
		ctx = handler.OnEnd(ctx, runInfo, output)
	}

	return ctx, output
}

// OnWithStreamHandle 通用流式处理器触发逻辑
// - 复制流副本以串联处理，按给定 handle 将流传递给各 handler
func OnWithStreamHandle[S any](
	ctx context.Context,
	inOut S,
	handlers []Handler,
	cpy func(int) []S,
	handle func(context.Context, Handler, S) context.Context) (context.Context, S) {

	if len(handlers) == 0 {
		return ctx, inOut
	}

	inOuts := cpy(len(handlers) + 1)

	for i, handler := range handlers {
		ctx = handle(ctx, handler, inOuts[i])
	}

	return ctx, inOuts[len(inOuts)-1]
}

// OnStartWithStreamInputHandle 触发流式 OnStart（输入流）
func OnStartWithStreamInputHandle[T any](ctx context.Context, input *schema.StreamReader[T],
	runInfo *RunInfo, handlers []Handler) (context.Context, *schema.StreamReader[T]) {

	handlers = generic.Reverse(handlers)

	cpy := input.Copy

	handle := func(ctx context.Context, handler Handler, in *schema.StreamReader[T]) context.Context {
		in_ := schema.StreamReaderWithConvert(in, func(i T) (CallbackInput, error) {
			return i, nil
		})
		return handler.OnStartWithStreamInput(ctx, runInfo, in_)
	}

	return OnWithStreamHandle(ctx, input, handlers, cpy, handle)
}

// OnEndWithStreamOutputHandle 触发流式 OnEnd（输出流）
func OnEndWithStreamOutputHandle[T any](ctx context.Context, output *schema.StreamReader[T],
	runInfo *RunInfo, handlers []Handler) (context.Context, *schema.StreamReader[T]) {

	cpy := output.Copy

	handle := func(ctx context.Context, handler Handler, out *schema.StreamReader[T]) context.Context {
		out_ := schema.StreamReaderWithConvert(out, func(i T) (CallbackOutput, error) {
			return i, nil
		})
		return handler.OnEndWithStreamOutput(ctx, runInfo, out_)
	}

	return OnWithStreamHandle(ctx, output, handlers, cpy, handle)
}

// OnErrorHandle 触发错误处理器（顺序调用）
func OnErrorHandle(ctx context.Context, err error,
	runInfo *RunInfo, handlers []Handler) (context.Context, error) {

	for _, handler := range handlers {
		ctx = handler.OnError(ctx, runInfo, err)
	}

	return ctx, err
}
