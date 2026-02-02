/*
 * Copyright 2026 CloudWeGo Authors
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
	"errors"
	"fmt"

	"github.com/cloudwego/eino/internal/core"
)

// Interrupt pauses tool execution and signals the orchestration layer to checkpoint.
// The tool can be resumed later with optional data.
//
// Parameters:
//   - ctx: The context passed to InvokableRun/StreamableRun
//   - info: User-facing information about why the tool is interrupting (e.g., "needs user confirmation")
//
// Returns an error that should be returned from InvokableRun/StreamableRun.
//
// Example:
//
//	func (t *MyTool) InvokableRun(ctx context.Context, args string, opts ...Option) (string, error) {
//	    if needsConfirmation(args) {
//	        return "", tool.Interrupt(ctx, "Please confirm this action")
//	    }
//	    return doWork(args), nil
//	}
//
// Interrupt 暂停工具执行并向编排层发送检查点信号。
// 工具稍后可以使用可选数据恢复。
//
// 参数：
//   - ctx: 传递给 InvokableRun/StreamableRun 的上下文
//   - info: 关于工具为何中断的用户可见信息（例如，“需要用户确认”）
//
// 返回应从 InvokableRun/StreamableRun 返回的错误。
//
// 示例：
//
//	func (t *MyTool) InvokableRun(ctx context.Context, args string, opts ...Option) (string, error) {
//	    if needsConfirmation(args) {
//	        return "", tool.Interrupt(ctx, "Please confirm this action")
//	    }
//	    return doWork(args), nil
//	}
func Interrupt(ctx context.Context, info any) error {
	is, err := core.Interrupt(ctx, info, nil, nil)
	if err != nil {
		return err
	}
	return is
}

// StatefulInterrupt pauses tool execution with state preservation.
// Use this when the tool has internal state that must be restored on resume.
//
// Parameters:
//   - ctx: The context passed to InvokableRun/StreamableRun
//   - info: User-facing information about the interrupt
//   - state: Internal state to persist (must be gob-serializable)
//
// Example:
//
//	func (t *MyTool) InvokableRun(ctx context.Context, args string, opts ...Option) (string, error) {
//	    wasInterrupted, hasState, state := tool.GetInterruptState[MyState](ctx)
//	    if !wasInterrupted {
//	        // First run - interrupt with state
//	        return "", tool.StatefulInterrupt(ctx, "processing", MyState{Step: 1})
//	    }
//	    // Resumed - continue from saved state
//	    return continueFrom(state), nil
//	}
//
// StatefulInterrupt 暂停工具执行并保留状态。
// 当工具具有必须在恢复时恢复的内部状态时使用此方法。
//
// 参数：
//   - ctx: 传递给 InvokableRun/StreamableRun 的上下文
//   - info: 关于中断的用户可见信息
//   - state: 要持久化的内部状态（必须可 gob 序列化）
//
// 示例：
//
//	func (t *MyTool) InvokableRun(ctx context.Context, args string, opts ...Option) (string, error) {
//	    wasInterrupted, hasState, state := tool.GetInterruptState[MyState](ctx)
//	    if !wasInterrupted {
//	        // 第一次运行 - 带状态中断
//	        return "", tool.StatefulInterrupt(ctx, "processing", MyState{Step: 1})
//	    }
//	    // 已恢复 - 从保存的状态继续
//	    return continueFrom(state), nil
//	}
func StatefulInterrupt(ctx context.Context, info any, state any) error {
	is, err := core.Interrupt(ctx, info, state, nil)
	if err != nil {
		return err
	}
	return is
}

// CompositeInterrupt creates an interrupt that aggregates multiple sub-interrupts.
// Use this when a tool internally executes a graph or other interruptible components.
//
// Parameters:
//   - ctx: The context passed to InvokableRun/StreamableRun
//   - info: User-facing information for this tool's interrupt
//   - state: Internal state to persist for this tool
//   - errs: Interrupt errors from sub-components (graphs, other tools, etc.)
//
// Example:
//
//	func (t *MyTool) InvokableRun(ctx context.Context, args string, opts ...Option) (string, error) {
//	    result, err := t.internalGraph.Invoke(ctx, input)
//	    if err != nil {
//	        if _, ok := tool.IsInterruptError(err); ok {
//	            return "", tool.CompositeInterrupt(ctx, "graph interrupted", myState, err)
//	        }
//	        return "", err
//	    }
//	    return result, nil
//	}
//
// CompositeInterrupt 创建一个聚合多个子中断的中断。
// 当工具在内部执行图或其他可中断组件时使用此方法。
//
// 参数：
//   - ctx: 传递给 InvokableRun/StreamableRun 的上下文
//   - info: 此工具中断的用户可见信息
//   - state: 此工具要持久化的内部状态
//   - errs: 来自子组件（图、其他工具等）的中断错误
//
// 示例：
//
//	func (t *MyTool) InvokableRun(ctx context.Context, args string, opts ...Option) (string, error) {
//	    result, err := t.internalGraph.Invoke(ctx, input)
//	    if err != nil {
//	        if _, ok := tool.IsInterruptError(err); ok {
//	            return "", tool.CompositeInterrupt(ctx, "graph interrupted", myState, err)
//	        }
//	        return "", err
//	    }
//	    return result, nil
//	}
func CompositeInterrupt(ctx context.Context, info any, state any, errs ...error) error {
	if len(errs) == 0 {
		return StatefulInterrupt(ctx, info, state)
	}

	var cErrs []*core.InterruptSignal
	for _, err := range errs {
		ire := &core.InterruptSignal{}
		if errors.As(err, &ire) {
			cErrs = append(cErrs, ire)
			continue
		}

		var provider core.InterruptContextsProvider
		if errors.As(err, &provider) {
			is := core.FromInterruptContexts(provider.GetInterruptContexts())
			if is != nil {
				cErrs = append(cErrs, is)
			}
			continue
		}

		return fmt.Errorf("composite interrupt but one of the sub error is not interrupt error: %w", err)
	}

	is, err := core.Interrupt(ctx, info, state, cErrs)
	if err != nil {
		return err
	}
	return is
}

// GetInterruptState checks if the tool was previously interrupted and retrieves saved state.
//
// Returns:
//   - wasInterrupted: true if this tool was part of a previous interruption
//   - hasState: true if state was saved and successfully cast to type T
//   - state: the saved state (zero value if hasState is false)
//
// Example:
//
//	func (t *MyTool) InvokableRun(ctx context.Context, args string, opts ...Option) (string, error) {
//	    wasInterrupted, hasState, state := tool.GetInterruptState[MyState](ctx)
//	    if wasInterrupted && hasState {
//	        // Continue from saved state
//	        return continueFrom(state), nil
//	    }
//	    // First run
//	    return "", tool.StatefulInterrupt(ctx, "need input", MyState{Step: 1})
//	}
func GetInterruptState[T any](ctx context.Context) (wasInterrupted bool, hasState bool, state T) {
	return core.GetInterruptState[T](ctx)
}

// GetResumeContext checks if this tool is the explicit target of a resume operation.
//
// Returns:
//   - isResumeTarget: true if this tool was explicitly targeted for resume
//   - hasData: true if resume data was provided
//   - data: the resume data (zero value if hasData is false)
//
// Use this to differentiate between:
//   - Being resumed as the target (should proceed with work)
//   - Being re-executed because a sibling was resumed (should re-interrupt)
//
// Example:
//
//	func (t *MyTool) InvokableRun(ctx context.Context, args string, opts ...Option) (string, error) {
//	    wasInterrupted, _, _ := tool.GetInterruptState[any](ctx)
//	    if !wasInterrupted {
//	        return "", tool.Interrupt(ctx, "need confirmation")
//	    }
//
//	    isTarget, hasData, data := tool.GetResumeContext[string](ctx)
//	    if !isTarget {
//	        // Not our turn - re-interrupt
//	        return "", tool.Interrupt(ctx, nil)
//	    }
//	    if hasData {
//	        return data, nil
//	    }
//	    return "default result", nil
//	}
func GetResumeContext[T any](ctx context.Context) (isResumeTarget bool, hasData bool, data T) {
	return core.GetResumeContext[T](ctx)
}
