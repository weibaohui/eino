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
	"errors"
	"runtime/debug"
	"sync"

	"github.com/cloudwego/eino/internal/safe"
	"github.com/cloudwego/eino/schema"
)

func init() {
	schema.RegisterName[*deterministicTransferState]("_eino_adk_deterministic_transfer_state")
}

// deterministicTransferState 用于存储确定性转移的中断状态。
type deterministicTransferState struct {
	EventList []*agentEventWrapper
}

// AgentWithDeterministicTransferTo 包装一个 Agent，使其在执行结束后确定性地转移到指定的 Agents。
// 为什么要做这个：在某些场景下，我们希望 Agent 执行完后，总是将控制权交给特定的一组 Agent，而不是由模型决定。
// 如何使用：传入上下文和 DeterministicTransferConfig（包含目标 Agent 列表）。
func AgentWithDeterministicTransferTo(_ context.Context, config *DeterministicTransferConfig) Agent {
	if ra, ok := config.Agent.(ResumableAgent); ok {
		return &resumableAgentWithDeterministicTransferTo{
			agent:        ra,
			toAgentNames: config.ToAgentNames,
		}
	}
	return &agentWithDeterministicTransferTo{
		agent:        config.Agent,
		toAgentNames: config.ToAgentNames,
	}
}

// agentWithDeterministicTransferTo 是不支持 Resume 的包装实现。
type agentWithDeterministicTransferTo struct {
	agent        Agent
	toAgentNames []string
}

func (a *agentWithDeterministicTransferTo) Description(ctx context.Context) string {
	return a.agent.Description(ctx)
}

func (a *agentWithDeterministicTransferTo) Name(ctx context.Context) string {
	return a.agent.Name(ctx)
}

// Run 执行 Agent，并在执行结束后发送转移事件。
// 逻辑：
// 1. 如果是 flowAgent，使用隔离会话运行。
// 2. 否则，直接运行内部 Agent。
// 3. 启动一个 goroutine 转发内部 Agent 的事件，并在最后追加 TransferToAgent 事件。
func (a *agentWithDeterministicTransferTo) Run(ctx context.Context,
	input *AgentInput, options ...AgentRunOption) *AsyncIterator[*AgentEvent] {

	if fa, ok := a.agent.(*flowAgent); ok {
		return runFlowAgentWithIsolatedSession(ctx, fa, input, a.toAgentNames, options...)
	}

	aIter := a.agent.Run(ctx, input, options...)

	iterator, generator := NewAsyncIteratorPair[*AgentEvent]()
	go forwardEventsAndAppendTransfer(aIter, generator, a.toAgentNames)

	return iterator
}

// resumableAgentWithDeterministicTransferTo 是支持 Resume 的包装实现。
type resumableAgentWithDeterministicTransferTo struct {
	agent        ResumableAgent
	toAgentNames []string
}

func (a *resumableAgentWithDeterministicTransferTo) Description(ctx context.Context) string {
	return a.agent.Description(ctx)
}

func (a *resumableAgentWithDeterministicTransferTo) Name(ctx context.Context) string {
	return a.agent.Name(ctx)
}

func (a *resumableAgentWithDeterministicTransferTo) Run(ctx context.Context,
	input *AgentInput, options ...AgentRunOption) *AsyncIterator[*AgentEvent] {

	if fa, ok := a.agent.(*flowAgent); ok {
		return runFlowAgentWithIsolatedSession(ctx, fa, input, a.toAgentNames, options...)
	}

	aIter := a.agent.Run(ctx, input, options...)

	iterator, generator := NewAsyncIteratorPair[*AgentEvent]()
	go forwardEventsAndAppendTransfer(aIter, generator, a.toAgentNames)

	return iterator
}

// Resume 恢复 Agent 执行，并在执行结束后发送转移事件。
func (a *resumableAgentWithDeterministicTransferTo) Resume(ctx context.Context, info *ResumeInfo, opts ...AgentRunOption) *AsyncIterator[*AgentEvent] {
	if fa, ok := a.agent.(*flowAgent); ok {
		return resumeFlowAgentWithIsolatedSession(ctx, fa, info, a.toAgentNames, opts...)
	}

	aIter := a.agent.Resume(ctx, info, opts...)

	iterator, generator := NewAsyncIteratorPair[*AgentEvent]()
	go forwardEventsAndAppendTransfer(aIter, generator, a.toAgentNames)

	return iterator
}

// forwardEventsAndAppendTransfer 转发事件流，并在流结束或中断前追加转移事件。
// 逻辑：
// 1. 遍历输入迭代器，将事件发送到生成器。
// 2. 检查最后一个事件是否是中断或退出，如果是，则不发送转移事件。
// 3. 否则，发送 TransferToAgent 事件，将控制权转移给指定的目标 Agent。
func forwardEventsAndAppendTransfer(iter *AsyncIterator[*AgentEvent],
	generator *AsyncGenerator[*AgentEvent], toAgentNames []string) {

	defer func() {
		if panicErr := recover(); panicErr != nil {
			generator.Send(&AgentEvent{Err: safe.NewPanicErr(panicErr, debug.Stack())})
		}
		generator.Close()
	}()

	var lastEvent *AgentEvent
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		generator.Send(event)
		lastEvent = event
	}

	if lastEvent != nil && lastEvent.Action != nil && (lastEvent.Action.Interrupted != nil || lastEvent.Action.Exit) {
		return
	}

	sendTransferEvents(generator, toAgentNames)
}

// runFlowAgentWithIsolatedSession 在隔离的会话中运行 flowAgent。
// 为什么要做这个：flowAgent 可能有自己的状态管理，需要与父会话隔离，避免污染。
func runFlowAgentWithIsolatedSession(ctx context.Context, fa *flowAgent, input *AgentInput,
	toAgentNames []string, options ...AgentRunOption) *AsyncIterator[*AgentEvent] {

	parentSession := getSession(ctx)
	parentRunCtx := getRunCtx(ctx)

	isolatedSession := &runSession{
		Values:    parentSession.Values,
		valuesMtx: parentSession.valuesMtx,
	}
	if isolatedSession.valuesMtx == nil {
		isolatedSession.valuesMtx = &sync.Mutex{}
	}
	if isolatedSession.Values == nil {
		isolatedSession.Values = make(map[string]any)
	}

	ctx = setRunCtx(ctx, &runContext{
		Session:   isolatedSession,
		RootInput: parentRunCtx.RootInput,
		RunPath:   parentRunCtx.RunPath,
	})

	iter := fa.Run(ctx, input, options...)

	iterator, generator := NewAsyncIteratorPair[*AgentEvent]()
	go handleFlowAgentEvents(ctx, iter, generator, isolatedSession, parentSession, toAgentNames)

	return iterator
}

func resumeFlowAgentWithIsolatedSession(ctx context.Context, fa *flowAgent, info *ResumeInfo,
	toAgentNames []string, opts ...AgentRunOption) *AsyncIterator[*AgentEvent] {

	state, ok := info.InterruptState.(*deterministicTransferState)
	if !ok || state == nil {
		return genErrorIter(errors.New("invalid interrupt state for flowAgent resume in deterministic transfer"))
	}

	parentSession := getSession(ctx)
	parentRunCtx := getRunCtx(ctx)

	isolatedSession := &runSession{
		Values:    parentSession.Values,
		valuesMtx: parentSession.valuesMtx,
		Events:    state.EventList,
	}
	if isolatedSession.valuesMtx == nil {
		isolatedSession.valuesMtx = &sync.Mutex{}
	}
	if isolatedSession.Values == nil {
		isolatedSession.Values = make(map[string]any)
	}

	ctx = setRunCtx(ctx, &runContext{
		Session:   isolatedSession,
		RootInput: parentRunCtx.RootInput,
		RunPath:   parentRunCtx.RunPath,
	})

	iter := fa.Resume(ctx, info, opts...)

	iterator, generator := NewAsyncIteratorPair[*AgentEvent]()
	go handleFlowAgentEvents(ctx, iter, generator, isolatedSession, parentSession, toAgentNames)

	return iterator
}

func handleFlowAgentEvents(ctx context.Context, iter *AsyncIterator[*AgentEvent],
	generator *AsyncGenerator[*AgentEvent], isolatedSession, parentSession *runSession, toAgentNames []string) {

	defer func() {
		if panicErr := recover(); panicErr != nil {
			generator.Send(&AgentEvent{Err: safe.NewPanicErr(panicErr, debug.Stack())})
		}
		generator.Close()
	}()

	var lastEvent *AgentEvent

	for {
		event, ok := iter.Next()
		if !ok {
			break
		}

		if parentSession != nil && (event.Action == nil || event.Action.Interrupted == nil) {
			copied := copyAgentEvent(event)
			setAutomaticClose(copied)
			setAutomaticClose(event)
			parentSession.addEvent(copied)
		}

		if event.Action != nil && event.Action.internalInterrupted != nil {
			lastEvent = event
			continue
		}

		generator.Send(event)
		lastEvent = event
	}

	if lastEvent != nil && lastEvent.Action != nil {
		if lastEvent.Action.internalInterrupted != nil {
			events := isolatedSession.getEvents()
			state := &deterministicTransferState{EventList: events}
			compositeEvent := CompositeInterrupt(ctx, "deterministic transfer wrapper interrupted",
				state, lastEvent.Action.internalInterrupted)
			generator.Send(compositeEvent)
			return
		}

		if lastEvent.Action.Exit {
			return
		}
	}

	sendTransferEvents(generator, toAgentNames)
}

func sendTransferEvents(generator *AsyncGenerator[*AgentEvent], toAgentNames []string) {
	for _, toAgentName := range toAgentNames {
		aMsg, tMsg := GenTransferMessages(context.Background(), toAgentName)

		aEvent := EventFromMessage(aMsg, nil, schema.Assistant, "")
		generator.Send(aEvent)

		tEvent := EventFromMessage(tMsg, nil, schema.Tool, tMsg.ToolName)
		tEvent.Action = &AgentAction{
			TransferToAgent: &TransferToAgentAction{
				DestAgentName: toAgentName,
			},
		}
		generator.Send(tEvent)
	}
}
