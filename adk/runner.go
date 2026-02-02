/*
 * Copyright 2025 CloudWeGo Authors
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

package adk

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/cloudwego/eino/internal/core"
	"github.com/cloudwego/eino/internal/safe"
	"github.com/cloudwego/eino/schema"
)

// Runner 是执行 Agent 的主要入口点。
// 它管理 Agent 的生命周期，包括启动、恢复和 Checkpoint。
// 为什么要做这个：提供一个高层的接口来运行 Agent，处理复杂的流式传输、状态持久化和中断恢复逻辑，使用户无需直接操作底层接口。
// 如何使用：通过 NewRunner 创建，然后调用 Run、Query 或 Resume 系列方法。
type Runner struct {
	// a 是要执行的目标 Agent。
	a Agent
	// enableStreaming 指示是否以流式模式执行。
	enableStreaming bool
	// store 是 Checkpoint 存储接口，用于在中断时持久化状态。如果为 nil，则禁用 Checkpoint 功能。
	store CheckPointStore
}

type CheckPointStore = core.CheckPointStore

// RunnerConfig 包含创建 Runner 的配置。
type RunnerConfig struct {
	// Agent 要运行的 Agent 实例。
	Agent Agent
	// EnableStreaming 是否启用流式输出。
	EnableStreaming bool

	// CheckPointStore 可选的状态持久化存储。
	CheckPointStore CheckPointStore
}

// ResumeParams contains all parameters needed to resume an execution.
// This struct provides an extensible way to pass resume parameters without
// requiring breaking changes to method signatures.
// ResumeParams 包含恢复执行所需的所有参数。
// 此结构提供了一种可扩展的方式来传递恢复参数，而无需对方法签名进行重大更改。
type ResumeParams struct {
	// Targets contains the addresses of components to be resumed as keys,
	// with their corresponding resume data as values
	// Targets 包含要恢复的组件地址作为键，其相应的恢复数据作为值
	Targets map[string]any
	// Future extensible fields can be added here without breaking changes
	// 未来可在此处添加可扩展字段，而不会破坏更改
}

// NewRunner creates a Runner that executes an Agent with optional streaming
// and checkpoint persistence.
// NewRunner 创建一个 Runner，用于执行具有可选流式传输和 Checkpoint 持久化的 Agent。
func NewRunner(_ context.Context, conf RunnerConfig) *Runner {
	return &Runner{
		enableStreaming: conf.EnableStreaming,
		a:               conf.Agent,
		store:           conf.CheckPointStore,
	}
}

// Run starts a new execution of the agent with a given set of messages.
// It returns an iterator that yields agent events as they occur.
// If the Runner was configured with a CheckPointStore, it will automatically save the agent's state
// upon interruption.
// Run 使用给定的一组消息开始 Agent 的新执行。
// 它返回一个迭代器，该迭代器在 Agent 事件发生时产生这些事件。
// 如果 Runner 配置了 CheckPointStore，它将在中断时自动保存 Agent 的状态。
func (r *Runner) Run(ctx context.Context, messages []Message,
	opts ...AgentRunOption) *AsyncIterator[*AgentEvent] {
	o := getCommonOptions(nil, opts...)

	fa := toFlowAgent(ctx, r.a)

	input := &AgentInput{
		Messages:        messages,
		EnableStreaming: r.enableStreaming,
	}

	ctx = ctxWithNewRunCtx(ctx, input, o.sharedParentSession)

	AddSessionValues(ctx, o.sessionValues)

	iter := fa.Run(ctx, input, opts...)
	if r.store == nil {
		return iter
	}

	niter, gen := NewAsyncIteratorPair[*AgentEvent]()

	go r.handleIter(ctx, iter, gen, o.checkPointID)
	return niter
}

// Query is a convenience method that starts a new execution with a single user query string.
// Query 是一个便捷方法，使用单个用户查询字符串开始新执行。
func (r *Runner) Query(ctx context.Context,
	query string, opts ...AgentRunOption) *AsyncIterator[*AgentEvent] {

	return r.Run(ctx, []Message{schema.UserMessage(query)}, opts...)
}

// Resume continues an interrupted execution from a checkpoint, using an "Implicit Resume All" strategy.
// This method is best for simpler use cases where the act of resuming implies that all previously
// interrupted points should proceed without specific data.
//
// When using this method, all interrupted agents will receive `isResumeFlow = false` when they
// call `GetResumeContext`, as no specific agent was targeted. This is suitable for the "Simple Confirmation"
// pattern where an agent only needs to know `wasInterrupted` is true to continue.
// Resume 使用“隐式全部恢复”策略从 Checkpoint 继续中断的执行。
// 此方法最适合简单的用例，即恢复操作意味着所有先前中断的点都应在没有特定数据的情况下继续。
//
// 当使用此方法时，所有被中断的 Agent 在调用 `GetResumeContext` 时都将收到 `isResumeFlow = false`，
// 因为没有针对特定的 Agent。这适用于“简单确认”模式，其中 Agent 只需要知道 `wasInterrupted` 为 true 即可继续。
func (r *Runner) Resume(ctx context.Context, checkPointID string, opts ...AgentRunOption) (
	*AsyncIterator[*AgentEvent], error) {
	return r.resume(ctx, checkPointID, nil, opts...)
}

// ResumeWithParams continues an interrupted execution from a checkpoint with specific parameters.
// This is the most common and powerful way to resume, allowing you to target specific interrupt points
// (identified by their address/ID) and provide them with data.
//
// The params.Targets map should contain the addresses of the components to be resumed as keys. These addresses
// can point to any interruptible component in the entire execution graph, including ADK agents, compose
// graph nodes, or tools. The value can be the resume data for that component, or `nil` if no data is needed.
//
// When using this method:
//   - Components whose addresses are in the params.Targets map will receive `isResumeFlow = true` when they
//     call `GetResumeContext`.
//   - Interrupted components whose addresses are NOT in the params.Targets map must decide how to proceed:
//     -- "Leaf" components (the actual root causes of the original interrupt) MUST re-interrupt themselves
//     to preserve their state.
//     -- "Composite" agents (like SequentialAgent or ChatModelAgent) should generally proceed with their
//     execution. They act as conduits, allowing the resume signal to flow to their children. They will
//     naturally re-interrupt if one of their interrupted children re-interrupts, as they receive the
//     new `CompositeInterrupt` signal from them.
//
// ResumeWithParams 使用特定参数从 Checkpoint 继续中断的执行。
// 这是最常见且功能强大的恢复方式，允许您针对特定的中断点（由其 Address/ID 标识）并为它们提供数据。
//
// params.Targets 映射应包含要恢复的组件的地址作为键。这些地址可以指向整个执行图中的任何可中断组件，
// 包括 ADK Agent、Compose 图节点或工具。该值可以是该组件的恢复数据，如果不需要数据，则为 `nil`。
//
// 使用此方法时：
//   - params.Targets 映射中包含其地址的组件在调用 `GetResumeContext` 时将收到 `isResumeFlow = true`。
//   - 地址不在 params.Targets 映射中的被中断组件必须决定如何继续：
//     -- “叶子”组件（原始中断的实际根本原因）必须重新中断自身以保留其状态。
//     -- “复合” Agent（如 SequentialAgent 或 ChatModelAgent）通常应继续执行。
//     它们充当管道，允许恢复信号流向其子级。如果它们的某个被中断子级重新中断，它们自然会重新中断，
//     因为它们会收到来自子级的新 `CompositeInterrupt` 信号。
func (r *Runner) ResumeWithParams(ctx context.Context, checkPointID string, params *ResumeParams, opts ...AgentRunOption) (*AsyncIterator[*AgentEvent], error) {
	return r.resume(ctx, checkPointID, params.Targets, opts...)
}

// resume is the internal implementation for both Resume and ResumeWithParams.
func (r *Runner) resume(ctx context.Context, checkPointID string, resumeData map[string]any,
	opts ...AgentRunOption) (*AsyncIterator[*AgentEvent], error) {
	if r.store == nil {
		return nil, fmt.Errorf("failed to resume: store is nil")
	}

	ctx, runCtx, resumeInfo, err := r.loadCheckPoint(ctx, checkPointID)
	if err != nil {
		return nil, fmt.Errorf("failed to load from checkpoint: %w", err)
	}

	o := getCommonOptions(nil, opts...)
	if o.sharedParentSession {
		parentSession := getSession(ctx)
		if parentSession != nil {
			runCtx.Session.Values = parentSession.Values
			runCtx.Session.valuesMtx = parentSession.valuesMtx
		}
	}
	if runCtx.Session.valuesMtx == nil {
		runCtx.Session.valuesMtx = &sync.Mutex{}
	}
	if runCtx.Session.Values == nil {
		runCtx.Session.Values = make(map[string]any)
	}

	ctx = setRunCtx(ctx, runCtx)

	AddSessionValues(ctx, o.sessionValues)

	if len(resumeData) > 0 {
		ctx = core.BatchResumeWithData(ctx, resumeData)
	}

	fa := toFlowAgent(ctx, r.a)
	aIter := fa.Resume(ctx, resumeInfo, opts...)
	if r.store == nil {
		return aIter, nil
	}

	niter, gen := NewAsyncIteratorPair[*AgentEvent]()

	go r.handleIter(ctx, aIter, gen, &checkPointID)
	return niter, nil
}

func (r *Runner) handleIter(ctx context.Context, aIter *AsyncIterator[*AgentEvent],
	gen *AsyncGenerator[*AgentEvent], checkPointID *string) {
	defer func() {
		panicErr := recover()
		if panicErr != nil {
			e := safe.NewPanicErr(panicErr, debug.Stack())
			gen.Send(&AgentEvent{Err: e})
		}

		gen.Close()
	}()
	var (
		interruptSignal *core.InterruptSignal
		legacyData      any
	)
	for {
		event, ok := aIter.Next()
		if !ok {
			break
		}

		if event.Action != nil && event.Action.internalInterrupted != nil {
			if interruptSignal != nil {
				// even if multiple interrupt happens, they should be merged into one
				// action by CompositeInterrupt, so here in Runner we must assume at most
				// one interrupt action happens
				panic("multiple interrupt actions should not happen in Runner")
			}
			interruptSignal = event.Action.internalInterrupted
			interruptContexts := core.ToInterruptContexts(interruptSignal, allowedAddressSegmentTypes)
			event = &AgentEvent{
				AgentName: event.AgentName,
				RunPath:   event.RunPath,
				Output:    event.Output,
				Action: &AgentAction{
					Interrupted: &InterruptInfo{
						Data:              event.Action.Interrupted.Data,
						InterruptContexts: interruptContexts,
					},
					internalInterrupted: interruptSignal,
				},
			}
			legacyData = event.Action.Interrupted.Data

			if checkPointID != nil {
				// save checkpoint first before sending interrupt event,
				// so when end-user receives interrupt event, they can resume from this checkpoint
				err := r.saveCheckPoint(ctx, *checkPointID, &InterruptInfo{
					Data: legacyData,
				}, interruptSignal)
				if err != nil {
					gen.Send(&AgentEvent{Err: fmt.Errorf("failed to save checkpoint: %w", err)})
				}
			}
		}

		gen.Send(event)
	}
}
