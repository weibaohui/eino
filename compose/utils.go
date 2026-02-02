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

package compose

import (
	"context"
	"fmt"
	"reflect"

	"github.com/cloudwego/eino/callbacks"
	icb "github.com/cloudwego/eino/internal/callbacks"
	"github.com/cloudwego/eino/internal/generic"
	"github.com/cloudwego/eino/schema"
)

// on is a generic callback function type.
// on 是一个泛型回调函数类型。
type on[T any] func(context.Context, T) (context.Context, T)

// onStart invokes the OnStart callbacks.
// onStart 调用 OnStart 回调。
func onStart[T any](ctx context.Context, input T) (context.Context, T) {
	return icb.On(ctx, input, icb.OnStartHandle[T], callbacks.TimingOnStart, true)
}

// onEnd invokes the OnEnd callbacks.
// onEnd 调用 OnEnd 回调。
func onEnd[T any](ctx context.Context, output T) (context.Context, T) {
	return icb.On(ctx, output, icb.OnEndHandle[T], callbacks.TimingOnEnd, false)
}

// onStartWithStreamInput invokes the OnStartWithStreamInput callbacks.
// onStartWithStreamInput 调用 OnStartWithStreamInput 回调。
func onStartWithStreamInput[T any](ctx context.Context, input *schema.StreamReader[T]) (
	context.Context, *schema.StreamReader[T]) {

	return icb.On(ctx, input, icb.OnStartWithStreamInputHandle[T], callbacks.TimingOnStartWithStreamInput, true)
}

// genericOnStartWithStreamInputHandle handles the OnStartWithStreamInput callback for generic types.
// genericOnStartWithStreamInputHandle 处理泛型类型的 OnStartWithStreamInput 回调。
func genericOnStartWithStreamInputHandle(ctx context.Context, input streamReader,
	runInfo *icb.RunInfo, handlers []icb.Handler) (context.Context, streamReader) {

	handlers = generic.Reverse(handlers)

	cpy := input.copy

	handle := func(ctx context.Context, handler icb.Handler, in streamReader) context.Context {
		in_, ok := unpackStreamReader[icb.CallbackInput](in)
		if !ok {
			panic("impossible")
		}

		return handler.OnStartWithStreamInput(ctx, runInfo, in_)
	}

	return icb.OnWithStreamHandle(ctx, input, handlers, cpy, handle)
}

// genericOnStartWithStreamInput invokes the OnStartWithStreamInput callback for generic types.
// genericOnStartWithStreamInput 调用泛型类型的 OnStartWithStreamInput 回调。
func genericOnStartWithStreamInput(ctx context.Context, input streamReader) (context.Context, streamReader) {
	return icb.On(ctx, input, genericOnStartWithStreamInputHandle, callbacks.TimingOnStartWithStreamInput, true)
}

// onEndWithStreamOutput invokes the OnEndWithStreamOutput callbacks.
// onEndWithStreamOutput 调用 OnEndWithStreamOutput 回调。
func onEndWithStreamOutput[T any](ctx context.Context, output *schema.StreamReader[T]) (
	context.Context, *schema.StreamReader[T]) {

	return icb.On(ctx, output, icb.OnEndWithStreamOutputHandle[T], callbacks.TimingOnEndWithStreamOutput, false)
}

// genericOnEndWithStreamOutputHandle handles the OnEndWithStreamOutput callback for generic types.
// genericOnEndWithStreamOutputHandle 处理泛型类型的 OnEndWithStreamOutput 回调。
func genericOnEndWithStreamOutputHandle(ctx context.Context, output streamReader,
	runInfo *icb.RunInfo, handlers []icb.Handler) (context.Context, streamReader) {

	cpy := output.copy

	handle := func(ctx context.Context, handler icb.Handler, out streamReader) context.Context {
		out_, ok := unpackStreamReader[icb.CallbackOutput](out)
		if !ok {
			panic("impossible")
		}

		return handler.OnEndWithStreamOutput(ctx, runInfo, out_)
	}

	return icb.OnWithStreamHandle(ctx, output, handlers, cpy, handle)
}

// genericOnEndWithStreamOutput invokes the OnEndWithStreamOutput callback for generic types.
// genericOnEndWithStreamOutput 调用泛型类型的 OnEndWithStreamOutput 回调。
func genericOnEndWithStreamOutput(ctx context.Context, output streamReader) (context.Context, streamReader) {
	return icb.On(ctx, output, genericOnEndWithStreamOutputHandle, callbacks.TimingOnEndWithStreamOutput, false)
}

// onError invokes the OnError callbacks.
// onError 调用 OnError 回调。
func onError(ctx context.Context, err error) (context.Context, error) {
	return icb.On(ctx, err, icb.OnErrorHandle, callbacks.TimingOnError, false)
}

// runWithCallbacks executes a function with lifecycle callbacks.
// It handles OnStart, OnEnd, and OnError callbacks automatically.
// runWithCallbacks 执行带有生命周期回调的函数。
// 它自动处理 OnStart、OnEnd 和 OnError 回调。
func runWithCallbacks[I, O, TOption any](r func(context.Context, I, ...TOption) (O, error),
	onStart on[I], onEnd on[O], onError on[error]) func(context.Context, I, ...TOption) (O, error) {

	return func(ctx context.Context, input I, opts ...TOption) (output O, err error) {
		ctx, input = onStart(ctx, input)

		output, err = r(ctx, input, opts...)
		if err != nil {
			ctx, err = onError(ctx, err)
			return output, err
		}

		ctx, output = onEnd(ctx, output)

		return output, nil
	}
}

// invokeWithCallbacks wraps an Invoke function with callbacks.
// invokeWithCallbacks 使用回调包装 Invoke 函数。
func invokeWithCallbacks[I, O, TOption any](i Invoke[I, O, TOption]) Invoke[I, O, TOption] {
	return runWithCallbacks(i, onStart[I], onEnd[O], onError)
}

// onGraphStart invokes the OnStart callback for the graph.
// onGraphStart 调用图的 OnStart 回调。
func onGraphStart(ctx context.Context, input any, isStream bool) (context.Context, any) {
	if isStream {
		return genericOnStartWithStreamInput(ctx, input.(streamReader))
	}
	return onStart(ctx, input)
}

// onGraphEnd invokes the OnEnd callback for the graph.
// onGraphEnd 调用图的 OnEnd 回调。
func onGraphEnd(ctx context.Context, output any, isStream bool) (context.Context, any) {
	if isStream {
		return genericOnEndWithStreamOutput(ctx, output.(streamReader))
	}
	return onEnd(ctx, output)
}

// onGraphError invokes the OnError callback for the graph.
// onGraphError 调用图的 OnError 回调。
func onGraphError(ctx context.Context, err error) (context.Context, error) {
	return onError(ctx, err)
}

// streamWithCallbacks wraps a Stream function with callbacks.
// streamWithCallbacks 使用回调包装 Stream 函数。
func streamWithCallbacks[I, O, TOption any](s Stream[I, O, TOption]) Stream[I, O, TOption] {
	return runWithCallbacks(s, onStart[I], onEndWithStreamOutput[O], onError)
}

// collectWithCallbacks wraps a Collect function with callbacks.
// collectWithCallbacks 使用回调包装 Collect 函数。
func collectWithCallbacks[I, O, TOption any](c Collect[I, O, TOption]) Collect[I, O, TOption] {
	return runWithCallbacks(c, onStartWithStreamInput[I], onEnd[O], onError)
}

// transformWithCallbacks wraps a Transform function with callbacks.
// transformWithCallbacks 使用回调包装 Transform 函数。
func transformWithCallbacks[I, O, TOption any](t Transform[I, O, TOption]) Transform[I, O, TOption] {
	return runWithCallbacks(t, onStartWithStreamInput[I], onEndWithStreamOutput[O], onError)
}

// initGraphCallbacks initializes callbacks for the graph.
// initGraphCallbacks 初始化图的回调。
func initGraphCallbacks(ctx context.Context, info *nodeInfo, meta *executorMeta, opts ...Option) context.Context {
	ri := &callbacks.RunInfo{}
	if meta != nil {
		ri.Component = meta.component
		ri.Type = meta.componentImplType
	}

	if info != nil {
		ri.Name = info.name
	}

	var cbs []callbacks.Handler
	for i := range opts {
		if len(opts[i].handler) != 0 && len(opts[i].paths) == 0 {
			cbs = append(cbs, opts[i].handler...)
		}
	}

	if len(cbs) == 0 {
		return icb.ReuseHandlers(ctx, ri)
	}

	return icb.AppendHandlers(ctx, ri, cbs...)
}

// initNodeCallbacks initializes callbacks for a node.
// initNodeCallbacks 初始化节点的回调。
func initNodeCallbacks(ctx context.Context, key string, info *nodeInfo, meta *executorMeta, opts ...Option) context.Context {
	ri := &callbacks.RunInfo{}
	if meta != nil {
		ri.Component = meta.component
		ri.Type = meta.componentImplType
	}

	if info != nil {
		ri.Name = info.name
	}

	var cbs []callbacks.Handler
	for i := range opts {
		if len(opts[i].handler) != 0 {
			if len(opts[i].paths) != 0 {
				for _, k := range opts[i].paths {
					if len(k.path) == 1 && k.path[0] == key {
						cbs = append(cbs, opts[i].handler...)
						break
					}
				}
			}
		}
	}

	if len(cbs) == 0 {
		return icb.ReuseHandlers(ctx, ri)
	}

	return icb.AppendHandlers(ctx, ri, cbs...)
}

// streamChunkConvertForCBOutput converts a stream chunk for callback output.
// streamChunkConvertForCBOutput 转换回调输出的流块。
func streamChunkConvertForCBOutput[O any](o O) (callbacks.CallbackOutput, error) {
	return o, nil
}

// streamChunkConvertForCBInput converts a stream chunk for callback input.
// streamChunkConvertForCBInput 转换回调输入的流块。
func streamChunkConvertForCBInput[I any](i I) (callbacks.CallbackInput, error) {
	return i, nil
}

// toAnyList converts a typed slice to a slice of any.
// toAnyList 将类型化切片转换为 any 切片。
func toAnyList[T any](in []T) []any {
	ret := make([]any, len(in))
	for i := range in {
		ret[i] = in[i]
	}
	return ret
}

// assignableType represents the assignability of types.
// assignableType 表示类型的可赋值性。
type assignableType uint8

const (
	assignableTypeMustNot assignableType = iota
	assignableTypeMust
	assignableTypeMay
)

// checkAssignable checks if the input type is assignable to the argument type.
// checkAssignable 检查输入类型是否可赋值给参数类型。
func checkAssignable(input, arg reflect.Type) assignableType {
	if arg == nil || input == nil {
		return assignableTypeMustNot
	}

	if arg == input {
		return assignableTypeMust
	}

	if arg.Kind() == reflect.Interface && input.Implements(arg) {
		return assignableTypeMust
	}
	if input.Kind() == reflect.Interface {
		if arg.Implements(input) {
			return assignableTypeMay
		}
		return assignableTypeMustNot
	}

	return assignableTypeMustNot
}

// extractOption extracts options for nodes from the provided options.
// extractOption 从提供的选项中提取节点的选项。
func extractOption(nodes map[string]*chanCall, opts ...Option) (map[string][]any, error) {
	optMap := map[string][]any{}
	for _, opt := range opts {
		if len(opt.paths) == 0 {
			// common, discard callback, filter option by type
			if len(opt.options) == 0 {
				continue
			}
			for name, c := range nodes {
				if c.action.optionType == nil {
					// subgraph
					optMap[name] = append(optMap[name], opt)
				} else if reflect.TypeOf(opt.options[0]) == c.action.optionType { // assume that types of options are the same
					optMap[name] = append(optMap[name], opt.options...)
				}
			}
		}
		for _, path := range opt.paths {
			if len(path.path) == 0 {
				return nil, fmt.Errorf("call option has designated an empty path")
			}

			var curNode *chanCall
			var ok bool
			if curNode, ok = nodes[path.path[0]]; !ok {
				return nil, fmt.Errorf("option has designated an unknown node: %s", path)
			}
			curNodeKey := path.path[0]

			if len(path.path) == 1 {
				if len(opt.options) == 0 {
					// sub graph common callbacks has been added to ctx in initNodeCallback and won't be passed to subgraph only pass options
					// node callback also won't be passed
					continue
				}
				if curNode.action.optionType == nil {
					nOpt := opt.deepCopy()
					nOpt.paths = []*NodePath{}
					optMap[curNodeKey] = append(optMap[curNodeKey], nOpt)
				} else {
					// designate to component
					if curNode.action.optionType != reflect.TypeOf(opt.options[0]) { // assume that types of options are the same
						return nil, fmt.Errorf("option type[%s] is different from which the designated node[%s] expects[%s]",
							reflect.TypeOf(opt.options[0]).String(), path, curNode.action.optionType.String())
					}
					optMap[curNodeKey] = append(optMap[curNodeKey], opt.options...)
				}
			} else {
				if curNode.action.optionType != nil {
					// component
					return nil, fmt.Errorf("cannot designate sub path of a component, path:%s", path)
				}
				// designate to sub graph's nodes
				nOpt := opt.deepCopy()
				nOpt.paths = []*NodePath{NewNodePath(path.path[1:]...)}
				optMap[curNodeKey] = append(optMap[curNodeKey], nOpt)
			}
		}
	}

	return optMap, nil
}

// mapToList converts a map to a list of its values.
// mapToList 将 map 转换为其值的列表。
func mapToList(m map[string]any) []any {
	ret := make([]any, 0, len(m))
	for _, v := range m {
		ret = append(ret, v)
	}
	return ret
}
