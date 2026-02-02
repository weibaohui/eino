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
	"io"
	"strings"

	"github.com/google/uuid"

	"github.com/cloudwego/eino/internal"
	"github.com/cloudwego/eino/schema"
)

// AsyncIterator 是一个异步迭代器，用于从通道中接收数据。
// 为什么要做这个：提供一种异步遍历数据流的方式，支持无界通道。
type AsyncIterator[T any] struct {
	// ch 是底层的无界通道。
	ch *internal.UnboundedChan[T]
}

// Next 获取下一个元素。
// 返回值：
//   - T: 获取到的元素。
//   - bool: 如果通道已关闭且无更多元素，则返回 false；否则返回 true。
func (ai *AsyncIterator[T]) Next() (T, bool) {
	return ai.ch.Receive()
}

// AsyncGenerator 是一个异步生成器，用于向通道发送数据。
// 为什么要做这个：提供一种异步生成数据流的方式，与 AsyncIterator 配对使用。
type AsyncGenerator[T any] struct {
	// ch 是底层的无界通道。
	ch *internal.UnboundedChan[T]
}

// Send 发送一个元素到通道。
// 参数：
//   - v: 要发送的元素。
func (ag *AsyncGenerator[T]) Send(v T) {
	ag.ch.Send(v)
}

// Close 关闭生成器，表示不再发送更多数据。
func (ag *AsyncGenerator[T]) Close() {
	ag.ch.Close()
}

// NewAsyncIteratorPair returns a paired async iterator and generator
// that share the same underlying channel.
// NewAsyncIteratorPair 返回一对共享相同底层通道的异步迭代器和生成器。
// 为什么要做这个：方便创建一个生产者-消费者模型的数据流管道。
// 如何使用：调用此函数获取迭代器和生成器，生成器用于发送数据，迭代器用于接收数据。
func NewAsyncIteratorPair[T any]() (*AsyncIterator[T], *AsyncGenerator[T]) {
	ch := internal.NewUnboundedChan[T]()
	return &AsyncIterator[T]{ch}, &AsyncGenerator[T]{ch}
}

// copyMap 复制一个 map。
// 为什么要做这个：在需要修改 map 但不影响原始 map 时使用，或为了并发安全进行快照。
func copyMap[K comparable, V any](m map[K]V) map[K]V {
	res := make(map[K]V, len(m))
	for k, v := range m {
		res[k] = v
	}
	return res
}

// concatInstructions 连接多条指令字符串。
// 为什么要做这个：将多个指令片段合并为一个完整的指令字符串，中间用空行分隔。
func concatInstructions(instructions ...string) string {
	var sb strings.Builder
	sb.WriteString(instructions[0])
	for i := 1; i < len(instructions); i++ {
		sb.WriteString("\n\n")
		sb.WriteString(instructions[i])
	}

	return sb.String()
}

// GenTransferMessages generates assistant and tool messages to instruct a
// transfer-to-agent tool call targeting the destination agent.
// GenTransferMessages 生成 Assistant 消息和工具消息，以指示针对目标 Agent 的转交工具调用。
// 为什么要做这个：在多 Agent 协作中，当需要从当前 Agent 转移到另一个 Agent 时，需要构造特定的消息序列来触发转移。
// 如何使用：传入上下文和目标 Agent 名称，返回构造好的 Assistant 消息和 Tool 消息。
// 代码逻辑：生成一个唯一的 ToolCallID，构造一个调用 transferToAgent 工具的 ToolCall，以及对应的 ToolMessage。
func GenTransferMessages(_ context.Context, destAgentName string) (Message, Message) {
	toolCallID := uuid.NewString()
	tooCall := schema.ToolCall{ID: toolCallID, Function: schema.FunctionCall{Name: TransferToAgentToolName, Arguments: destAgentName}}
	assistantMessage := schema.AssistantMessage("", []schema.ToolCall{tooCall})
	toolMessage := schema.ToolMessage(transferToAgentToolOutput(destAgentName), toolCallID, schema.WithToolName(TransferToAgentToolName))
	return assistantMessage, toolMessage
}

// setAutomaticClose 设置 AgentEvent 的消息流自动关闭。
// 为什么要做这个：确保在流式输出结束时，底层资源被正确释放。
// 代码逻辑：检查 Output 是否为流式，如果是，则设置 MessageStream 的自动关闭标志。
func setAutomaticClose(e *AgentEvent) {
	if e.Output == nil || e.Output.MessageOutput == nil || !e.Output.MessageOutput.IsStreaming {
		return
	}

	e.Output.MessageOutput.MessageStream.SetAutomaticClose()
}

// getMessageFromWrappedEvent extracts the message from an AgentEvent.
// If the stream contains an error chunk, this function returns (nil, err) and
// sets StreamErr to prevent re-consumption. The nil message ensures that
// failed stream responses are not included in subsequent agents' context windows.
// getMessageFromWrappedEvent 从 AgentEvent 中提取消息。
// 如果流包含错误块，此函数返回 (nil, err) 并设置 StreamErr 以防止重新消费。
// nil 消息确保失败的流响应不包含在后续 Agent 的上下文窗口中。
func getMessageFromWrappedEvent(e *agentEventWrapper) (Message, error) {
	if e.AgentEvent.Output == nil || e.AgentEvent.Output.MessageOutput == nil {
		return nil, nil
	}

	if !e.AgentEvent.Output.MessageOutput.IsStreaming {
		return e.AgentEvent.Output.MessageOutput.Message, nil
	}

	if e.concatenatedMessage != nil {
		return e.concatenatedMessage, nil
	}

	if e.StreamErr != nil {
		return nil, e.StreamErr
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	if e.concatenatedMessage != nil {
		return e.concatenatedMessage, nil
	}

	var (
		msgs []Message
		s    = e.AgentEvent.Output.MessageOutput.MessageStream
	)

	defer s.Close()
	for {
		msg, err := s.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			e.StreamErr = err
			// Replace the stream with successfully received messages only (no error at the end).
			// The error is preserved in StreamErr for users to check.
			// We intentionally exclude the error from the new stream to ensure gob encoding
			// compatibility, as the stream may be consumed during serialization.
			e.AgentEvent.Output.MessageOutput.MessageStream = schema.StreamReaderFromArray(msgs)
			return nil, err
		}

		msgs = append(msgs, msg)
	}

	if len(msgs) == 0 {
		return nil, errors.New("no messages in MessageVariant.MessageStream")
	}

	if len(msgs) == 1 {
		e.concatenatedMessage = msgs[0]
	} else {
		var err error
		e.concatenatedMessage, err = schema.ConcatMessages(msgs)
		if err != nil {
			e.StreamErr = err
			e.AgentEvent.Output.MessageOutput.MessageStream = schema.StreamReaderFromArray(msgs)
			return nil, err
		}
	}

	return e.concatenatedMessage, nil
}

// copyAgentEvent copies an AgentEvent.
// If the MessageVariant is streaming, the MessageStream will be copied.
// RunPath will be deep copied.
// The result of Copy will be a new AgentEvent that is:
// - safe to set fields of AgentEvent
// - safe to extend RunPath
// - safe to receive from MessageStream
// NOTE: even if the AgentEvent is copied, it's still not recommended to modify
// the Message itself or Chunks of the MessageStream, as they are not copied.
// NOTE: if you have CustomizedOutput or CustomizedAction, they are NOT copied.
// copyAgentEvent 复制一个 AgentEvent。
// 如果 MessageVariant 是流式的，则 MessageStream 将被复制。
// RunPath 将被深拷贝。
// Copy 的结果将是一个新的 AgentEvent，它是：
// - 安全地设置 AgentEvent 的字段
// - 安全地扩展 RunPath
// - 安全地从 MessageStream 接收
// 注意：即使 AgentEvent 被复制，仍然不建议修改 Message 本身或 MessageStream 的 Chunk，因为它们没有被复制。
// 注意：如果您有 CustomizedOutput 或 CustomizedAction，它们不会被复制。
func copyAgentEvent(ae *AgentEvent) *AgentEvent {
	rp := make([]RunStep, len(ae.RunPath))
	copy(rp, ae.RunPath)

	copied := &AgentEvent{
		AgentName: ae.AgentName,
		RunPath:   rp,
		Action:    ae.Action,
		Err:       ae.Err,
	}

	if ae.Output == nil {
		return copied
	}

	copied.Output = &AgentOutput{
		CustomizedOutput: ae.Output.CustomizedOutput,
	}

	mv := ae.Output.MessageOutput
	if mv == nil {
		return copied
	}

	copied.Output.MessageOutput = &MessageVariant{
		IsStreaming: mv.IsStreaming,
		Role:        mv.Role,
		ToolName:    mv.ToolName,
	}
	if mv.IsStreaming {
		sts := ae.Output.MessageOutput.MessageStream.Copy(2)
		mv.MessageStream = sts[0]
		copied.Output.MessageOutput.MessageStream = sts[1]
	} else {
		copied.Output.MessageOutput.Message = mv.Message
	}

	return copied
}

// GetMessage extracts the Message from an AgentEvent. For streaming output,
// it duplicates the stream and concatenates it into a single Message.
// GetMessage 从 AgentEvent 中提取消息。对于流式输出，它会复制流并将其连接成单个消息。
// 为什么要做这个：方便获取完整消息内容，同时保留原始 AgentEvent 中的流可供后续使用。
// 代码逻辑：如果是流式输出，分叉流，一个用于拼接返回，一个放回 AgentEvent；如果是非流式，直接返回消息。
func GetMessage(e *AgentEvent) (Message, *AgentEvent, error) {
	if e.Output == nil || e.Output.MessageOutput == nil {
		return nil, e, nil
	}

	msgOutput := e.Output.MessageOutput
	if msgOutput.IsStreaming {
		ss := msgOutput.MessageStream.Copy(2)
		e.Output.MessageOutput.MessageStream = ss[0]

		msg, err := schema.ConcatMessageStream(ss[1])

		return msg, e, err
	}

	return msgOutput.Message, e, nil
}

// genErrorIter 生成一个包含错误的异步迭代器。
// 为什么要做这个：方便在发生错误时返回一个符合接口要求的迭代器，其中包含错误信息。
func genErrorIter(err error) *AsyncIterator[*AgentEvent] {
	iterator, generator := NewAsyncIteratorPair[*AgentEvent]()
	generator.Send(&AgentEvent{Err: err})
	generator.Close()
	return iterator
}
