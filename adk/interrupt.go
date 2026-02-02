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
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"

	"github.com/cloudwego/eino/internal/core"
	"github.com/cloudwego/eino/schema"
)

// ResumeInfo holds all the information necessary to resume an interrupted agent execution.
// It is created by the framework and passed to an agent's Resume method.
// ResumeInfo 包含恢复被中断 Agent 执行所需的所有信息。
// 它由框架创建，并传递给 Agent 的 Resume 方法。
// 为什么要做这个：在 Agent 被中断后，重新运行（Resume）时需要知道原始执行的模式、中断时的上下文、内部状态等，以便从中断点继续。
type ResumeInfo struct {
	// EnableStreaming indicates whether the original execution was in streaming mode.
	// EnableStreaming 指示原始执行是否处于流式模式。
	EnableStreaming bool

	// Deprecated: use InterruptContexts from the embedded InterruptInfo for user-facing details,
	// and GetInterruptState for internal state retrieval.
	// Deprecated: 请使用嵌入的 InterruptInfo 中的 InterruptContexts 获取面向用户的详细信息，
	// 并使用 GetInterruptState 获取内部状态。
	*InterruptInfo

	// WasInterrupted 指示当前 Agent 是否被直接中断（而不是其子 Agent 被中断）。
	WasInterrupted bool
	// InterruptState 中断时保存的内部状态数据。
	InterruptState any
	// IsResumeTarget 指示当前 Agent 是否是恢复执行的目标。
	IsResumeTarget bool
	// ResumeData 恢复执行时传入的数据（例如用户的进一步输入）。
	ResumeData any
}

// InterruptInfo contains all the information about an interruption event.
// It is created by the framework when an agent returns an interrupt action.
// InterruptInfo 包含有关中断事件的所有信息。
// 当 Agent 返回中断 Action 时，由框架创建。
// 为什么要做这个：统一封装中断相关的信息，包括面向用户的数据和中断链上下文。
type InterruptInfo struct {
	// Data 关联的中断数据，通常是面向用户的信息。
	Data any

	// InterruptContexts provides a structured, user-facing view of the interrupt chain.
	// Each context represents a step in the agent hierarchy that was interrupted.
	// InterruptContexts 提供中断链的结构化、面向用户的视图。
	// 每个上下文代表被中断的 Agent 层级中的一个步骤。
	InterruptContexts []*InterruptCtx
}

// Interrupt creates a basic interrupt action.
// This is used when an agent needs to pause its execution to request external input or intervention,
// but does not need to save any internal state to be restored upon resumption.
// The `info` parameter is user-facing data that describes the reason for the interrupt.
// Interrupt 创建一个基本的中断 Action。
// 当 Agent 需要暂停执行以请求外部输入或干预，但不需要保存任何内部状态以在恢复时还原时使用此方法。
// `info` 参数是面向用户的数据，描述了中断的原因。
func Interrupt(ctx context.Context, info any) *AgentEvent {
	var rp []RunStep
	rCtx := getRunCtx(ctx)
	if rCtx != nil {
		rp = rCtx.RunPath
	}

	is, err := core.Interrupt(ctx, info, nil, nil,
		core.WithLayerPayload(rp))
	if err != nil {
		return &AgentEvent{Err: err}
	}

	contexts := core.ToInterruptContexts(is, allowedAddressSegmentTypes)

	return &AgentEvent{
		Action: &AgentAction{
			Interrupted: &InterruptInfo{
				InterruptContexts: contexts,
			},
			internalInterrupted: is,
		},
	}
}

// StatefulInterrupt creates an interrupt action that also saves the agent's internal state.
// This is used when an agent has internal state that must be restored for it to continue correctly.
// The `info` parameter is user-facing data describing the interrupt.
// The `state` parameter is the agent's internal state object, which will be serialized and stored.
// StatefulInterrupt 创建一个同时也保存 Agent 内部状态的中断 Action。
// 当 Agent 具有必须还原才能正确继续的内部状态时使用此方法。
// `info` 参数是描述中断的面向用户的数据。
// `state` 参数是 Agent 的内部状态对象，它将被序列化并存储。
func StatefulInterrupt(ctx context.Context, info any, state any) *AgentEvent {
	var rp []RunStep
	rCtx := getRunCtx(ctx)
	if rCtx != nil {
		rp = rCtx.RunPath
	}

	is, err := core.Interrupt(ctx, info, state, nil,
		core.WithLayerPayload(rp))
	if err != nil {
		return &AgentEvent{Err: err}
	}

	contexts := core.ToInterruptContexts(is, allowedAddressSegmentTypes)

	return &AgentEvent{
		Action: &AgentAction{
			Interrupted: &InterruptInfo{
				InterruptContexts: contexts,
			},
			internalInterrupted: is,
		},
	}
}

// CompositeInterrupt creates an interrupt action for a workflow agent.
// It combines the interrupts from one or more of its sub-agents into a single, cohesive interrupt.
// This is used by workflow agents (like Sequential, Parallel, or Loop) to propagate interrupts from their children.
// The `info` parameter is user-facing data describing the workflow's own reason for interrupting.
// The `state` parameter is the workflow agent's own state (e.g., the index of the sub-agent that was interrupted).
// The `subInterruptSignals` is a variadic list of the InterruptSignal objects from the interrupted sub-agents.
// CompositeInterrupt 为工作流 Agent 创建一个中断 Action。
// 它将来自一个或多个子 Agent 的中断组合成一个单一的、连贯的中断。
// 这由工作流 Agent（如 Sequential、Parallel 或 Loop）用于传播其子级的中断。
// `info` 参数是描述工作流自身中断原因的面向用户的数据。
// `state` 参数是工作流 Agent 自身的状态（例如，被中断的子 Agent 的索引）。
// `subInterruptSignals` 是来自被中断子 Agent 的 InterruptSignal 对象的可变参数列表。
func CompositeInterrupt(ctx context.Context, info any, state any,
	subInterruptSignals ...*InterruptSignal) *AgentEvent {
	var rp []RunStep
	rCtx := getRunCtx(ctx)
	if rCtx != nil {
		rp = rCtx.RunPath
	}

	is, err := core.Interrupt(ctx, info, state, subInterruptSignals,
		core.WithLayerPayload(rp))
	if err != nil {
		return &AgentEvent{Err: err}
	}

	contexts := core.ToInterruptContexts(is, allowedAddressSegmentTypes)

	return &AgentEvent{
		Action: &AgentAction{
			Interrupted: &InterruptInfo{
				InterruptContexts: contexts,
			},
			internalInterrupted: is,
		},
	}
}

// Address represents the unique, hierarchical address of a component within an execution.
// It is a slice of AddressSegments, where each segment represents one level of nesting.
// This is a type alias for core.Address. See the core package for more details.
// Address 表示执行中组件的唯一分层地址。
// 它是 AddressSegments 的切片，其中每个段代表一层嵌套。
// 这是 core.Address 的类型别名。有关更多详细信息，请参阅 core 包。
type Address = core.Address
type AddressSegment = core.AddressSegment
type AddressSegmentType = core.AddressSegmentType

const (
	AddressSegmentAgent AddressSegmentType = "agent"
	AddressSegmentTool  AddressSegmentType = "tool"
)

var allowedAddressSegmentTypes = []AddressSegmentType{AddressSegmentAgent, AddressSegmentTool}

// AppendAddressSegment adds an address segment for the current execution context.
// AppendAddressSegment 为当前执行上下文添加一个地址段。
func AppendAddressSegment(ctx context.Context, segType AddressSegmentType, segID string) context.Context {
	return core.AppendAddressSegment(ctx, segType, segID, "")
}

// InterruptCtx provides a structured, user-facing view of a single point of interruption.
// It contains the ID and Address of the interrupted component, as well as user-defined info.
// This is a type alias for core.InterruptCtx. See the core package for more details.
// InterruptCtx 提供单个中断点的结构化、面向用户的视图。
// 它包含被中断组件的 ID 和 Address，以及用户定义的信息。
// 这是 core.InterruptCtx 的类型别名。有关更多详细信息，请参阅 core 包。
type InterruptCtx = core.InterruptCtx
type InterruptSignal = core.InterruptSignal

// FromInterruptContexts converts user-facing interrupt contexts to an interrupt signal.
// FromInterruptContexts 将面向用户的中断上下文转换为中断信号。
func FromInterruptContexts(contexts []*InterruptCtx) *InterruptSignal {
	return core.FromInterruptContexts(contexts)
}

// WithCheckPointID sets the checkpoint ID used for interruption persistence.
// WithCheckPointID 设置用于中断持久化的 Checkpoint ID。
func WithCheckPointID(id string) AgentRunOption {
	return WrapImplSpecificOptFn(func(t *options) {
		t.checkPointID = &id
	})
}

func init() {
	schema.RegisterName[*serialization]("_eino_adk_serialization")
	schema.RegisterName[*WorkflowInterruptInfo]("_eino_adk_workflow_interrupt_info")
	schema.RegisterName[*State]("_eino_adk_react_state")
}

// serialization CheckpointSchema: root checkpoint payload (gob).
// Any type tagged with `CheckpointSchema:` is persisted and must remain backward compatible.
type serialization struct {
	RunCtx *runContext
	// deprecated: still keep it here for backward compatibility
	Info                *InterruptInfo
	EnableStreaming     bool
	InterruptID2Address map[string]Address
	InterruptID2State   map[string]core.InterruptState
}

// loadCheckPoint 从存储中加载 Checkpoint。
// 它解码序列化的数据，并恢复 RunContext 和 ResumeInfo。
func (r *Runner) loadCheckPoint(ctx context.Context, checkpointID string) (
	context.Context, *runContext, *ResumeInfo, error) {
	data, existed, err := r.store.Get(ctx, checkpointID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to get checkpoint from store: %w", err)
	}
	if !existed {
		return nil, nil, nil, fmt.Errorf("checkpoint[%s] not exist", checkpointID)
	}

	s := &serialization{}
	err = gob.NewDecoder(bytes.NewReader(data)).Decode(s)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to decode checkpoint: %w", err)
	}
	ctx = core.PopulateInterruptState(ctx, s.InterruptID2Address, s.InterruptID2State)

	return ctx, s.RunCtx, &ResumeInfo{
		EnableStreaming: s.EnableStreaming,
		InterruptInfo:   s.Info,
	}, nil
}

// saveCheckPoint 将 Checkpoint 保存到存储中。
// 它序列化 RunContext、中断信息和中断信号状态。
func (r *Runner) saveCheckPoint(
	ctx context.Context,
	key string,
	info *InterruptInfo,
	is *core.InterruptSignal,
) error {
	runCtx := getRunCtx(ctx)

	id2Addr, id2State := core.SignalToPersistenceMaps(is)

	buf := &bytes.Buffer{}
	err := gob.NewEncoder(buf).Encode(&serialization{
		RunCtx:              runCtx,
		Info:                info,
		InterruptID2Address: id2Addr,
		InterruptID2State:   id2State,
		EnableStreaming:     r.enableStreaming,
	})
	if err != nil {
		return fmt.Errorf("failed to encode checkpoint: %w", err)
	}
	return r.store.Set(ctx, key, buf.Bytes())
}

const bridgeCheckpointID = "adk_react_mock_key"

// newBridgeStore 创建一个新的 BridgeStore。
// BridgeStore 用于在测试或特殊场景中临时存储 Checkpoint 数据。
func newBridgeStore() *bridgeStore {
	return &bridgeStore{}
}

// newResumeBridgeStore 创建一个带有预设数据的 BridgeStore，用于恢复执行。
func newResumeBridgeStore(data []byte) *bridgeStore {
	return &bridgeStore{
		Data:  data,
		Valid: true,
	}
}

type bridgeStore struct {
	Data  []byte
	Valid bool
}

func (m *bridgeStore) Get(_ context.Context, _ string) ([]byte, bool, error) {
	if m.Valid {
		return m.Data, true, nil
	}
	return nil, false, nil
}

func (m *bridgeStore) Set(_ context.Context, _ string, checkPoint []byte) error {
	m.Data = checkPoint
	m.Valid = true
	return nil
}

func getNextResumeAgent(ctx context.Context, info *ResumeInfo) (string, error) {
	nextAgents, err := core.GetNextResumptionPoints(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get next agent leading to interruption: %w", err)
	}

	if len(nextAgents) == 0 {
		return "", errors.New("no child agents leading to interrupted agent were found")
	}

	if len(nextAgents) > 1 {
		return "", errors.New("agent has multiple child agents leading to interruption, " +
			"but concurrent transfer is not supported")
	}

	// get the single next agent to delegate to.
	var nextAgentID string
	for id := range nextAgents {
		nextAgentID = id
		break
	}

	return nextAgentID, nil
}

func getNextResumeAgents(ctx context.Context, info *ResumeInfo) (map[string]bool, error) {
	nextAgents, err := core.GetNextResumptionPoints(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get next agents leading to interruption: %w", err)
	}

	if len(nextAgents) == 0 {
		return nil, errors.New("no child agents leading to interrupted agent were found")
	}

	return nextAgents, nil
}

func buildResumeInfo(ctx context.Context, nextAgentID string, info *ResumeInfo) (
	context.Context, *ResumeInfo) {
	ctx = AppendAddressSegment(ctx, AddressSegmentAgent, nextAgentID)
	nextResumeInfo := &ResumeInfo{
		EnableStreaming: info.EnableStreaming,
		InterruptInfo:   info.InterruptInfo,
	}

	wasInterrupted, hasState, state := core.GetInterruptState[any](ctx)
	nextResumeInfo.WasInterrupted = wasInterrupted
	if hasState {
		nextResumeInfo.InterruptState = state
	}

	if wasInterrupted {
		isResumeTarget, hasData, data := core.GetResumeContext[any](ctx)
		nextResumeInfo.IsResumeTarget = isResumeTarget
		if hasData {
			nextResumeInfo.ResumeData = data
		}
	}

	ctx = updateRunPathOnly(ctx, nextAgentID)

	return ctx, nextResumeInfo
}
