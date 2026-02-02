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

import "context"

type CtxManagerKey struct{}
type CtxRunInfoKey struct{}

type manager struct {
	globalHandlers []Handler
	handlers       []Handler
	runInfo        *RunInfo
}

var GlobalHandlers []Handler

// newManager 创建回调管理器（结合全局与本地 handlers）
// - 返回值第二个参数表示是否存在任何处理器
func newManager(runInfo *RunInfo, handlers ...Handler) (*manager, bool) {
	if len(handlers)+len(GlobalHandlers) == 0 {
		return nil, false
	}

	hs := make([]Handler, len(GlobalHandlers))
	copy(hs, GlobalHandlers)

	return &manager{
		globalHandlers: hs,
		handlers:       handlers,
		runInfo:        runInfo,
	}, true
}

// ctxWithManager 将管理器注入上下文
func ctxWithManager(ctx context.Context, manager *manager) context.Context {
	return context.WithValue(ctx, CtxManagerKey{}, manager)
}

// withRunInfo 在现有管理器上改写 RunInfo（复制一份）
func (m *manager) withRunInfo(runInfo *RunInfo) *manager {
	if m == nil {
		return nil
	}

	n := *m
	n.runInfo = runInfo
	return &n
}

// managerFromCtx 从上下文获取管理器（若存在则复制返回）
func managerFromCtx(ctx context.Context) (*manager, bool) {
	v := ctx.Value(CtxManagerKey{})
	m, ok := v.(*manager)
	if ok && m != nil {
		n := *m
		return &n, true
	}

	return nil, false
}
