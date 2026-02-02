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
	"fmt"
	"io"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino/internal/callbacks"
	"github.com/cloudwego/eino/schema"
)

// TestAspectInject 测试切面注入功能。
// 验证 OnStart, OnEnd, OnError 以及流式输入输出的切面处理逻辑。
func TestAspectInject(t *testing.T) {
	t.Run("ctx without manager", func(t *testing.T) {
		ctx := context.Background()
		ctx = OnStart(ctx, 1)
		ctx = OnEnd(ctx, 2)
		ctx = OnError(ctx, fmt.Errorf("3"))
		isr, isw := schema.Pipe[int](2)
		go func() {
			for i := 0; i < 10; i++ {
				isw.Send(i, nil)
			}
			isw.Close()
		}()

		var nisr *schema.StreamReader[int]
		ctx, nisr = OnStartWithStreamInput(ctx, isr)
		j := 0
		for {
			i, err := nisr.Recv()
			if err == io.EOF {
				break
			}

			assert.NoError(t, err)
			assert.Equal(t, j, i)
			j++
		}
		nisr.Close()

		osr, osw := schema.Pipe[int](2)
		go func() {
			for i := 0; i < 10; i++ {
				osw.Send(i, nil)
			}
			osw.Close()
		}()

		var nosr *schema.StreamReader[int]
		ctx, nosr = OnEndWithStreamOutput(ctx, osr)
		j = 0
		for {
			i, err := nosr.Recv()
			if err == io.EOF {
				break
			}

			assert.NoError(t, err)
			assert.Equal(t, j, i)
			j++
		}
		nosr.Close()
	})

	t.Run("ctx with manager", func(t *testing.T) {
		ctx := context.Background()
		cnt := 0

		hb := NewHandlerBuilder().
			OnStartFn(func(ctx context.Context, info *RunInfo, input CallbackInput) context.Context {
				cnt += input.(int)
				return ctx
			}).
			OnEndFn(func(ctx context.Context, info *RunInfo, output CallbackOutput) context.Context {
				cnt += output.(int)
				return ctx
			}).
			OnErrorFn(func(ctx context.Context, info *RunInfo, err error) context.Context {
				v, _ := strconv.ParseInt(err.Error(), 10, 64)
				cnt += int(v)
				return ctx
			}).
			OnStartWithStreamInputFn(func(ctx context.Context, info *RunInfo, input *schema.StreamReader[CallbackInput]) context.Context {
				for {
					i, err := input.Recv()
					if err == io.EOF {
						break
					}

					cnt += i.(int)
				}

				input.Close()
				return ctx
			}).
			OnEndWithStreamOutputFn(func(ctx context.Context, info *RunInfo, output *schema.StreamReader[CallbackOutput]) context.Context {
				for {
					o, err := output.Recv()
					if err == io.EOF {
						break
					}

					cnt += o.(int)
				}

				output.Close()
				return ctx
			}).Build()

		ctx = InitCallbacks(ctx, nil, hb)

		ctx = OnStart(ctx, 1)
		ctx = OnEnd(ctx, 2)
		ctx = OnError(ctx, fmt.Errorf("3"))
		isr, isw := schema.Pipe[int](2)
		go func() {
			for i := 0; i < 10; i++ {
				isw.Send(i, nil)
			}
			isw.Close()
		}()

		ctx = ReuseHandlers(ctx, &RunInfo{})
		var nisr *schema.StreamReader[int]
		ctx, nisr = OnStartWithStreamInput(ctx, isr)
		j := 0
		for {
			i, err := nisr.Recv()
			if err == io.EOF {
				break
			}

			assert.NoError(t, err)
			assert.Equal(t, j, i)
			j++
			cnt += i
		}
		nisr.Close()

		osr, osw := schema.Pipe[int](2)
		go func() {
			for i := 0; i < 10; i++ {
				osw.Send(i, nil)
			}
			osw.Close()
		}()

		var nosr *schema.StreamReader[int]
		ctx, nosr = OnEndWithStreamOutput(ctx, osr)
		j = 0
		for {
			i, err := nosr.Recv()
			if err == io.EOF {
				break
			}

			assert.NoError(t, err)
			assert.Equal(t, j, i)
			j++
			cnt += i
		}
		nosr.Close()
		assert.Equal(t, 186, cnt)
	})
}

// TestGlobalCallbacksRepeated 测试全局回调是否重复执行。
// 验证当多次添加处理程序时，全局回调只执行一次（针对同一层级的调用）。
func TestGlobalCallbacksRepeated(t *testing.T) {
	times := 0
	testHandler := NewHandlerBuilder().OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
		times++
		return ctx
	}).Build()
	callbacks.GlobalHandlers = append(callbacks.GlobalHandlers, testHandler)

	ctx := context.Background()
	ctx = callbacks.AppendHandlers(ctx, &RunInfo{})
	ctx = callbacks.AppendHandlers(ctx, &RunInfo{})

	callbacks.On(ctx, "test", callbacks.OnStartHandle[string], TimingOnStart, true)
	assert.Equal(t, times, 1)
}

// TestEnsureRunInfo 测试 EnsureRunInfo 功能。
// 验证 EnsureRunInfo 能正确设置和更新 RunInfo，并且能正确处理全局回调的初始化。
func TestEnsureRunInfo(t *testing.T) {
	ctx := context.Background()

	var name, typ, comp string
	ctx = InitCallbacks(ctx, &RunInfo{Name: "name", Type: "type", Component: "component"}, NewHandlerBuilder().OnStartFn(func(ctx context.Context, info *RunInfo, input CallbackInput) context.Context {
		name = info.Name
		typ = info.Type
		comp = string(info.Component)
		return ctx
	}).Build())

	ctx = OnStart(ctx, "")
	assert.Equal(t, "name", name)
	assert.Equal(t, "type", typ)
	assert.Equal(t, "component", comp)
	ctx2 := EnsureRunInfo(ctx, "type2", "component2")
	OnStart(ctx2, "")
	assert.Equal(t, "", name)
	assert.Equal(t, "type2", typ)
	assert.Equal(t, "component2", comp)

	// EnsureRunInfo on an empty Context
	AppendGlobalHandlers(NewHandlerBuilder().OnStartFn(func(ctx context.Context, info *RunInfo, input CallbackInput) context.Context {
		typ = info.Type
		comp = string(info.Component)
		return ctx
	}).Build())
	ctx3 := EnsureRunInfo(context.Background(), "type3", "component3")
	OnStart(ctx3, 0)
	assert.Equal(t, "type3", typ)
	assert.Equal(t, "component3", comp)
	callbacks.GlobalHandlers = []Handler{}
}

// TestNesting 测试嵌套调用的场景。
// 验证在嵌套调用中，回调处理程序是否被正确复用或重新初始化。
func TestNesting(t *testing.T) {
	ctx := context.Background()
	cb := &myCallback{t: t}
	ctx = InitCallbacks(ctx, &RunInfo{
		Name: "test",
	}, cb)

	// jumped
	ctx1 := OnStart(ctx, 0)
	ctx2 := OnStart(ctx1, 1)
	OnEnd(ctx2, 1)
	OnEnd(ctx1, 0)
	assert.Equal(t, 4, cb.times)

	// reused
	cb.times = 0
	ctx1 = OnStart(ctx, 0)
	ctx2 = ReuseHandlers(ctx1, &RunInfo{Name: "test2"})
	ctx3 := OnStart(ctx2, 1)
	OnEnd(ctx3, 1)
	OnEnd(ctx1, 0)
	assert.Equal(t, 4, cb.times)

}

// TestReuseHandlersOnEmptyCtx 测试在空上下文上复用处理程序。
// 验证在空上下文上调用 ReuseHandlers 是否能正确初始化全局处理程序。
func TestReuseHandlersOnEmptyCtx(t *testing.T) {
	callbacks.GlobalHandlers = []Handler{}
	cb := &myCallback{t: t}
	AppendGlobalHandlers(cb)
	ctx := ReuseHandlers(context.Background(), &RunInfo{Name: "test"})
	OnStart(ctx, 0)
	assert.Equal(t, 1, cb.times)
}

// TestAppendHandlersTwiceOnSameCtx 测试在同一上下文上两次追加处理程序。
// 验证每次追加处理程序是否都创建了新的上下文，且互不影响。
func TestAppendHandlersTwiceOnSameCtx(t *testing.T) {
	callbacks.GlobalHandlers = []Handler{}
	cb := &myCallback{t: t}
	cb1 := &myCallback{t: t}
	cb2 := &myCallback{t: t}
	ctx := InitCallbacks(context.Background(), &RunInfo{Name: "test"}, cb)
	ctx1 := callbacks.AppendHandlers(ctx, &RunInfo{Name: "test"}, cb1)
	ctx2 := callbacks.AppendHandlers(ctx, &RunInfo{Name: "test"}, cb2)
	OnStart(ctx1, 0)
	OnStart(ctx2, 0)
	assert.Equal(t, 2, cb.times)
	assert.Equal(t, 1, cb1.times)
	assert.Equal(t, 1, cb2.times)
}

// myCallback 是用于测试的自定义回调实现。
type myCallback struct {
	t     *testing.T
	times int
}

func (m *myCallback) OnStart(ctx context.Context, info *RunInfo, input CallbackInput) context.Context {
	m.times++
	if info == nil {
		assert.Equal(m.t, 2, m.times)
		return ctx
	}
	if info.Name == "test" {
		assert.Equal(m.t, 0, input)
	} else {
		assert.Equal(m.t, 1, input)
	}

	return ctx
}

func (m *myCallback) OnEnd(ctx context.Context, info *RunInfo, output CallbackOutput) context.Context {
	m.times++
	if info == nil {
		assert.Equal(m.t, 3, m.times)
		return ctx
	}
	if info.Name == "test" {
		assert.Equal(m.t, 0, output)
	} else {
		assert.Equal(m.t, 1, output)
	}
	return ctx
}

func (m *myCallback) OnError(ctx context.Context, info *RunInfo, err error) context.Context {
	panic("implement me")
}

func (m *myCallback) OnStartWithStreamInput(ctx context.Context, info *RunInfo, input *schema.StreamReader[CallbackInput]) context.Context {
	panic("implement me")
}

func (m *myCallback) OnEndWithStreamOutput(ctx context.Context, info *RunInfo, output *schema.StreamReader[CallbackOutput]) context.Context {
	panic("implement me")
}
