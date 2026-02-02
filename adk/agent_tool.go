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

// Package adk provides core agent development kit utilities and types.
package adk

import (
	"context"
	"errors"
	"fmt"

	"github.com/bytedance/sonic"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

var (
	// defaultAgentToolParam 定义了默认的 Agent 工具参数 Schema。
	// 默认情况下，工具接受一个名为 "request" 的字符串参数。
	defaultAgentToolParam = schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
		"request": {
			Desc:     "request to be processed",
			Required: true,
			Type:     schema.String,
		},
	})
)

// AgentToolOptions 定义了 Agent 工具的配置选项。
type AgentToolOptions struct {
	// fullChatHistoryAsInput 指示是否使用完整的聊天记录作为输入。
	fullChatHistoryAsInput bool
	// agentInputSchema 自定义的 Agent 输入 Schema。
	agentInputSchema *schema.ParamsOneOf
}

// AgentToolOption 是用于配置 AgentToolOptions 的函数类型。
type AgentToolOption func(*AgentToolOptions)

// WithFullChatHistoryAsInput 启用使用完整的聊天记录作为输入。
// 如果启用，Agent 将接收当前的完整对话历史，而不仅仅是工具调用参数。
// 为什么要做这个：在某些场景下，被调用的 Agent 需要了解之前的上下文才能正确处理请求。
// 如何使用：在调用 NewAgentTool 时传入此选项。
func WithFullChatHistoryAsInput() AgentToolOption {
	return func(options *AgentToolOptions) {
		options.fullChatHistoryAsInput = true
	}
}

// WithAgentInputSchema 设置 Agent 工具的自定义输入 Schema。
// 为什么要做这个：默认的 Schema 只包含一个 "request" 字段，有时我们需要更复杂的输入结构。
// 如何使用：在调用 NewAgentTool 时传入自定义的 schema.ParamsOneOf。
func WithAgentInputSchema(schema *schema.ParamsOneOf) AgentToolOption {
	return func(options *AgentToolOptions) {
		options.agentInputSchema = schema
	}
}

func withAgentToolEnableStreaming(enabled bool) tool.Option {
	return tool.WrapImplSpecificOptFn(func(opt *agentToolOptions) {
		opt.enableStreaming = enabled
	})
}

// NewAgentTool 创建一个将 Agent 包装为可调用工具的 Tool。
//
// Event Streaming (事件流):
// 当在 ToolsConfig 中启用 EmitInternalEvents 时，Agent 工具会将内部 Agent 产生的 AgentEvent
// 发送到父 Agent 的 AsyncGenerator，从而允许通过 Runner 将内部 Agent 的输出实时流式传输给最终用户。
//
// 注意，这些转发的事件**不会**被记录在父 Agent 的 runSession 中。
// 它们仅被发送给最终用户，不会影响父 Agent 的状态或检查点。唯一的例外是 Interrupted 动作，
// 它会通过 CompositeInterrupt 传播，以实现跨 Agent 边界的正确中断/恢复。
//
// Action Scoping (动作作用域):
// 内部 Agent 发出的动作被限制在 Agent 工具的边界内：
//   - Interrupted: 通过 CompositeInterrupt 传播，允许跨边界正确中断/恢复
//   - Exit, TransferToAgent, BreakLoop: 在 Agent 工具外部被忽略；这些动作仅影响
//     内部 Agent 的执行，不会传播到父 Agent。
//
// 这种作用域限制确保了嵌套 Agent 不会意外终止或转移其父 Agent 的执行流。
//
// 为什么要做这个：允许将复杂的 Agent 封装为一个简单的工具，供其他 Agent 调用，实现多 Agent 协作。
// 如何使用：传入上下文、Agent 实例和可选的配置选项来创建工具。
func NewAgentTool(ctx context.Context, agent Agent, options ...AgentToolOption) tool.BaseTool {
	opts := &AgentToolOptions{}
	for _, opt := range options {
		opt(opts)
	}

	return &agentTool{
		agent:                  agent,
		fullChatHistoryAsInput: opts.fullChatHistoryAsInput,
		inputSchema:            opts.agentInputSchema,
	}
}

// agentTool 是 tool.BaseTool 的具体实现，用于包装 Agent。
type agentTool struct {
	agent Agent

	fullChatHistoryAsInput bool
	inputSchema            *schema.ParamsOneOf
}

// Info 获取工具的信息，包括名称、描述和参数 Schema。
// 为什么要做这个：工具调用方（如 ChatModel）需要这些信息来决定是否以及如何调用此工具。
func (at *agentTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	param := at.inputSchema
	if param == nil {
		param = defaultAgentToolParam
	}

	return &schema.ToolInfo{
		Name:        at.agent.Name(ctx),
		Desc:        at.agent.Description(ctx),
		ParamsOneOf: param,
	}, nil
}

// InvokableRun 执行工具调用逻辑。
// 它会启动内部 Agent，处理输入参数，并在需要时处理流式输出和中断恢复。
// 为什么要做这个：这是工具被调用时的实际执行入口。
func (at *agentTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	gen, enableStreaming := getEmitGeneratorAndEnableStreaming(opts)
	var ms *bridgeStore
	var iter *AsyncIterator[*AgentEvent]
	var err error

	wasInterrupted, hasState, state := tool.GetInterruptState[[]byte](ctx)
	if !wasInterrupted {
		ms = newBridgeStore()
		var input []Message
		if at.fullChatHistoryAsInput {
			input, err = getReactChatHistory(ctx, at.agent.Name(ctx))
			if err != nil {
				return "", err
			}
		} else {
			if at.inputSchema == nil {
				// default input schema
				type request struct {
					Request string `json:"request"`
				}

				req := &request{}
				err = sonic.UnmarshalString(argumentsInJSON, req)
				if err != nil {
					return "", err
				}
				argumentsInJSON = req.Request
			}
			input = []Message{
				schema.UserMessage(argumentsInJSON),
			}
		}

		iter = newInvokableAgentToolRunner(at.agent, ms, enableStreaming).Run(ctx, input,
			append(getOptionsByAgentName(at.agent.Name(ctx), opts), WithCheckPointID(bridgeCheckpointID), withSharedParentSession())...)
	} else {
		if !hasState {
			return "", fmt.Errorf("agent tool '%s' interrupt has happened, but cannot find interrupt state", at.agent.Name(ctx))
		}

		ms = newResumeBridgeStore(state)

		iter, err = newInvokableAgentToolRunner(at.agent, ms, enableStreaming).
			Resume(ctx, bridgeCheckpointID, append(getOptionsByAgentName(at.agent.Name(ctx), opts), withSharedParentSession())...)
		if err != nil {
			return "", err
		}
	}

	var lastEvent *AgentEvent
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}

		if lastEvent != nil &&
			lastEvent.Output != nil &&
			lastEvent.Output.MessageOutput != nil &&
			lastEvent.Output.MessageOutput.MessageStream != nil {
			lastEvent.Output.MessageOutput.MessageStream.Close()
		}

		if event.Err != nil {
			return "", event.Err
		}

		if gen != nil {
			if event.Action == nil || event.Action.Interrupted == nil {
				if parentRunCtx := getRunCtx(ctx); parentRunCtx != nil && len(parentRunCtx.RunPath) > 0 {
					rp := make([]RunStep, 0, len(parentRunCtx.RunPath)+len(event.RunPath))
					rp = append(rp, parentRunCtx.RunPath...)
					rp = append(rp, event.RunPath...)
					event.RunPath = rp
				}
				tmp := copyAgentEvent(event)
				gen.Send(event)
				event = tmp
			}
		}

		lastEvent = event
	}

	if lastEvent != nil && lastEvent.Action != nil && lastEvent.Action.Interrupted != nil {
		data, existed, err_ := ms.Get(ctx, bridgeCheckpointID)
		if err_ != nil {
			return "", fmt.Errorf("failed to get interrupt info: %w", err_)
		}
		if !existed {
			return "", fmt.Errorf("interrupt has happened, but cannot find interrupt info")
		}

		return "", tool.CompositeInterrupt(ctx, "agent tool interrupt", data,
			lastEvent.Action.internalInterrupted)
	}

	if lastEvent == nil {
		return "", errors.New("no event returned")
	}

	var ret string
	if lastEvent.Output != nil {
		if output := lastEvent.Output.MessageOutput; output != nil {
			msg, err := output.GetMessage()
			if err != nil {
				return "", err
			}
			ret = msg.Content
		}
	}

	return ret, nil
}

// agentToolOptions 是一个包装结构，用于将 AgentRunOption 切片转换为 tool.Option。
// 它存储了 Agent 名称和相应的运行选项，以便进行特定于工具的处理。
type agentToolOptions struct {
	agentName       string
	opts            []AgentRunOption
	generator       *AsyncGenerator[*AgentEvent]
	enableStreaming bool
}

// withAgentToolOptions 将 Agent 运行选项包装为 tool.Option。
// 为什么要做这个：允许在工具调用时传递 Agent 特定的配置。
func withAgentToolOptions(agentName string, opts []AgentRunOption) tool.Option {
	return tool.WrapImplSpecificOptFn(func(opt *agentToolOptions) {
		opt.agentName = agentName
		opt.opts = opts
	})
}

// withAgentToolEventGenerator 设置 Agent 工具的事件生成器。
// 为什么要做这个：允许将内部 Agent 的事件流式传输到外部。
func withAgentToolEventGenerator(gen *AsyncGenerator[*AgentEvent]) tool.Option {
	return tool.WrapImplSpecificOptFn(func(o *agentToolOptions) {
		o.generator = gen
	})
}

// getOptionsByAgentName 根据 Agent 名称获取相应的 AgentRunOption。
func getOptionsByAgentName(agentName string, opts []tool.Option) []AgentRunOption {
	var ret []AgentRunOption
	for _, opt := range opts {
		o := tool.GetImplSpecificOptions[agentToolOptions](nil, opt)
		if o != nil && o.agentName == agentName {
			ret = append(ret, o.opts...)
		}
	}
	return ret
}

// getEmitGeneratorAndEnableStreaming 从选项中获取事件生成器和流式启用状态。
func getEmitGeneratorAndEnableStreaming(opts []tool.Option) (*AsyncGenerator[*AgentEvent], bool) {
	o := tool.GetImplSpecificOptions[agentToolOptions](nil, opts...)
	if o == nil {
		return nil, false
	}

	return o.generator, o.enableStreaming
}

// getReactChatHistory 获取指定 Agent 的 React 聊天历史记录。
// 它会处理消息格式转换和角色重写。
func getReactChatHistory(ctx context.Context, destAgentName string) ([]Message, error) {
	var messages []Message
	var agentName string
	err := compose.ProcessState(ctx, func(ctx context.Context, st *State) error {
		messages = make([]Message, len(st.Messages)-1)
		copy(messages, st.Messages[:len(st.Messages)-1]) // remove the last assistant message, which is the tool call message
		agentName = st.AgentName
		return nil
	})

	a, t := GenTransferMessages(ctx, destAgentName)
	messages = append(messages, a, t)
	history := make([]Message, 0, len(messages))
	for _, msg := range messages {
		if msg.Role == schema.System {
			continue
		}

		if msg.Role == schema.Assistant || msg.Role == schema.Tool {
			msg = rewriteMessage(msg, agentName)
		}

		history = append(history, msg)
	}

	return history, err
}

// newInvokableAgentToolRunner 创建一个新的可调用 Agent 工具运行器。
func newInvokableAgentToolRunner(agent Agent, store compose.CheckPointStore, enableStreaming bool) *Runner {
	return &Runner{
		a:               agent,
		enableStreaming: enableStreaming,
		store:           store,
	}
}
