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
	"fmt"
	"io"

	"github.com/cloudwego/eino/internal/core"
	"github.com/cloudwego/eino/schema"
)

// Message 是 schema.Message 的指针别名。
type Message = *schema.Message

// MessageStream 是 schema.StreamReader[Message] 的指针别名。
type MessageStream = *schema.StreamReader[Message]

// MessageVariant 表示消息的变体，可以是普通消息，也可以是流式消息。
// 为什么要做这个：在处理流式输出时，我们需要一种统一的方式来传递消息内容，无论它是否完整。
type MessageVariant struct {
	// IsStreaming 指示当前消息是否为流式。
	IsStreaming bool

	Message       Message
	MessageStream MessageStream
	// message role: Assistant or Tool
	Role schema.RoleType
	// only used when Role is Tool
	ToolName string
}

// EventFromMessage 将消息或流封装为带有角色元数据的 AgentEvent。
// 为什么要做这个：方便快速构建包含消息输出的事件。
func EventFromMessage(msg Message, msgStream MessageStream,
	role schema.RoleType, toolName string) *AgentEvent {
	return &AgentEvent{
		Output: &AgentOutput{
			MessageOutput: &MessageVariant{
				IsStreaming:   msgStream != nil,
				Message:       msg,
				MessageStream: msgStream,
				Role:          role,
				ToolName:      toolName,
			},
		},
	}
}

type messageVariantSerialization struct {
	IsStreaming   bool
	Message       Message
	MessageStream Message
}

// GobEncode 实现 Gob 编码接口。
// 如果是流式消息，会先读取所有流内容并拼接成完整消息后再编码。
func (mv *MessageVariant) GobEncode() ([]byte, error) {
	s := &messageVariantSerialization{
		IsStreaming: mv.IsStreaming,
		Message:     mv.Message,
	}
	if mv.IsStreaming {
		var messages []Message
		for {
			frame, err := mv.MessageStream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("error receiving message stream: %w", err)
			}
			messages = append(messages, frame)
		}
		m, err := schema.ConcatMessages(messages)
		if err != nil {
			return nil, fmt.Errorf("failed to encode message: cannot concat message stream: %w", err)
		}
		s.MessageStream = m
	}
	buf := &bytes.Buffer{}
	err := gob.NewEncoder(buf).Encode(s)
	if err != nil {
		return nil, fmt.Errorf("failed to gob encode message variant: %w", err)
	}
	return buf.Bytes(), nil
}

// GobDecode 实现 Gob 解码接口。
func (mv *MessageVariant) GobDecode(b []byte) error {
	s := &messageVariantSerialization{}
	err := gob.NewDecoder(bytes.NewReader(b)).Decode(s)
	if err != nil {
		return fmt.Errorf("failed to decoding message variant: %w", err)
	}
	mv.IsStreaming = s.IsStreaming
	mv.Message = s.Message
	if s.MessageStream != nil {
		mv.MessageStream = schema.StreamReaderFromArray([]*schema.Message{s.MessageStream})
	}
	return nil
}

// GetMessage 获取完整的消息内容。
// 如果是流式消息，会读取流并拼接。
func (mv *MessageVariant) GetMessage() (Message, error) {
	var message Message
	if mv.IsStreaming {
		var err error
		message, err = schema.ConcatMessageStream(mv.MessageStream)
		if err != nil {
			return nil, err
		}
	} else {
		message = mv.Message
	}

	return message, nil
}

// TransferToAgentAction 表示转移到另一个 Agent 的动作。
type TransferToAgentAction struct {
	DestAgentName string
}

// AgentOutput 表示 Agent 的输出。
type AgentOutput struct {
	// MessageOutput 消息输出（可能是流式）。
	MessageOutput *MessageVariant

	// CustomizedOutput 自定义输出。
	CustomizedOutput any
}

// NewTransferToAgentAction 创建一个转移到指定 Agent 的动作。
// 为什么要做这个：显式地创建一个转移指令。
func NewTransferToAgentAction(destAgentName string) *AgentAction {
	return &AgentAction{TransferToAgent: &TransferToAgentAction{DestAgentName: destAgentName}}
}

// NewExitAction 创建一个退出动作。
// 为什么要做这个：指示 Agent 结束执行。
func NewExitAction() *AgentAction {
	return &AgentAction{Exit: true}
}

// AgentAction 表示 Agent 在执行期间可以发出的动作。
//
// Action Scoping in Agent Tools (Agent 工具中的动作作用域):
// 当一个 Agent 被包装为工具 (通过 NewAgentTool) 时，内部 Agent 发出的动作
// 被限制在工具边界内：
//   - Interrupted: 通过 CompositeInterrupt 传播，允许跨边界正确中断/恢复
//   - Exit, TransferToAgent, BreakLoop: 在 Agent 工具外部被忽略；这些动作仅影响
//     内部 Agent 的执行，不会传播到父 Agent。
//
// 这种作用域限制确保了嵌套 Agent 不会意外终止或转移其父 Agent 的执行流。
type AgentAction struct {
	// Exit 指示是否退出。
	Exit bool

	// Interrupted 中断信息。
	Interrupted *InterruptInfo

	// TransferToAgent 转移到其他 Agent 的动作。
	TransferToAgent *TransferToAgentAction

	// BreakLoop 跳出循环的动作。
	BreakLoop *BreakLoopAction

	// CustomizedAction 自定义动作。
	CustomizedAction any

	internalInterrupted *core.InterruptSignal
}

// RunStep 表示运行路径中的一个步骤，用于 Checkpoint 序列化。
type RunStep struct {
	agentName string
}

func init() {
	schema.RegisterName[[]RunStep]("eino_run_step_list")
}

func (r *RunStep) String() string {
	return r.agentName
}

func (r *RunStep) Equals(r1 RunStep) bool {
	return r.agentName == r1.agentName
}

func (r *RunStep) GobEncode() ([]byte, error) {
	s := &runStepSerialization{AgentName: r.agentName}
	buf := &bytes.Buffer{}
	err := gob.NewEncoder(buf).Encode(s)
	if err != nil {
		return nil, fmt.Errorf("failed to gob encode RunStep: %w", err)
	}
	return buf.Bytes(), nil
}

func (r *RunStep) GobDecode(b []byte) error {
	s := &runStepSerialization{}
	err := gob.NewDecoder(bytes.NewReader(b)).Decode(s)
	if err != nil {
		return fmt.Errorf("failed to gob decode RunStep: %w", err)
	}
	r.agentName = s.AgentName
	return nil
}

type runStepSerialization struct {
	AgentName string
}

// AgentEvent CheckpointSchema: persisted via serialization.RunCtx (gob).
type AgentEvent struct {
	AgentName string

	// RunPath represents the execution path from root agent to the current event source.
	// This field is managed entirely by the eino framework and cannot be set by end-users
	// because RunStep's fields are unexported. The framework sets RunPath exactly once:
	// - flowAgent sets it when the event has no RunPath (len == 0)
	// - agentTool prepends parent RunPath when forwarding events from nested agents
	RunPath []RunStep

	Output *AgentOutput

	Action *AgentAction

	Err error
}

type AgentInput struct {
	Messages        []Message
	EnableStreaming bool
}

//go:generate  mockgen -destination ../internal/mock/adk/Agent_mock.go --package adk -source interface.go
type Agent interface {
	Name(ctx context.Context) string
	Description(ctx context.Context) string

	// Run runs the agent.
	// The returned AgentEvent within the AsyncIterator must be safe to modify.
	// If the returned AgentEvent within the AsyncIterator contains MessageStream,
	// the MessageStream MUST be exclusive and safe to be received directly.
	// NOTE: it's recommended to use SetAutomaticClose() on the MessageStream of AgentEvents emitted by AsyncIterator,
	// so that even the events are not processed, the MessageStream can still be closed.
	Run(ctx context.Context, input *AgentInput, options ...AgentRunOption) *AsyncIterator[*AgentEvent]
}

type OnSubAgents interface {
	OnSetSubAgents(ctx context.Context, subAgents []Agent) error
	OnSetAsSubAgent(ctx context.Context, parent Agent) error

	OnDisallowTransferToParent(ctx context.Context) error
}

type ResumableAgent interface {
	Agent

	Resume(ctx context.Context, info *ResumeInfo, opts ...AgentRunOption) *AsyncIterator[*AgentEvent]
}
