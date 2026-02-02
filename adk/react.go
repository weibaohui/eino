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

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// ErrExceedMaxIterations indicates the agent reached the maximum iterations limit.
// ErrExceedMaxIterations 表示 Agent 达到了最大迭代次数限制。
var ErrExceedMaxIterations = errors.New("exceeds max iterations")

type adkToolResultSender func(ctx context.Context, toolName, callID, result string, prePopAction *AgentAction)
type adkStreamToolResultSender func(ctx context.Context, toolName, callID string, resultStream *schema.StreamReader[string], prePopAction *AgentAction)

type toolResultSenders struct {
	addr         Address
	sender       adkToolResultSender
	streamSender adkStreamToolResultSender
}

type toolResultSendersCtxKey struct{}

func setToolResultSendersToCtx(ctx context.Context, addr Address, sender adkToolResultSender, streamSender adkStreamToolResultSender) context.Context {
	return context.WithValue(ctx, toolResultSendersCtxKey{}, &toolResultSenders{
		addr:         addr,
		sender:       sender,
		streamSender: streamSender,
	})
}

func getToolResultSendersFromCtx(ctx context.Context) *toolResultSenders {
	v := ctx.Value(toolResultSendersCtxKey{})
	if v == nil {
		return nil
	}
	return v.(*toolResultSenders)
}

func isAddressAtDepth(currentAddr, handlerAddr Address, depth int) bool {
	expectedLen := len(handlerAddr) + depth
	return len(currentAddr) == expectedLen && currentAddr[:len(handlerAddr)].Equals(handlerAddr)
}

// State 保存 Agent 运行时状态，包括消息、工具操作和剩余迭代次数。
// 为什么要做这个：在 ReAct（Reasoning and Acting）循环中，需要跟踪对话历史、待执行的工具操作以及防止无限循环。
type State struct {
	// Messages 对话历史记录。
	Messages []Message

	// HasReturnDirectly 是否包含需要直接返回结果的工具调用。
	HasReturnDirectly bool
	// ReturnDirectlyToolCallID 需要直接返回结果的工具调用 ID。
	ReturnDirectlyToolCallID string

	// ToolGenActions 存储由工具产生的 AgentAction，用于在工具执行完后附加到事件中。
	ToolGenActions map[string]*AgentAction

	// AgentName 当前 Agent 的名称。
	AgentName string

	// RemainingIterations 剩余允许的迭代次数，防止死循环。
	RemainingIterations int
}

// SendToolGenAction attaches an AgentAction to the next tool event emitted for the
// current tool execution.
//
// Where/when to use:
//   - Invoke within a tool's Run (Invokable/Streamable) implementation to include
//     an action alongside that tool's output event.
//   - The action is scoped by the current tool call context: if a ToolCallID is
//     available, it is used as the key to support concurrent calls of the same
//     tool with different parameters; otherwise, the provided toolName is used.
//   - The stored action is ephemeral and will be popped and attached to the tool
//     event when the tool finishes (including streaming completion).
//
// Limitation:
//   - This function is intended for use within ChatModelAgent runs only. It relies
//     on ChatModelAgent's internal State to store and pop actions, which is not
//     available in other agent types.
//
// SendToolGenAction 将 AgentAction 附加到当前工具执行发出的下一个工具事件中。
//
// 何时/何地使用：
//   - 在工具的 Run (Invokable/Streamable) 实现中调用，以将 Action 包含在该工具的输出事件中。
//   - Action 的作用域受当前工具调用上下文限制：如果 ToolCallID 可用，它将被用作键以支持使用不同参数并发调用同一工具；否则，使用提供的 toolName。
//   - 存储的 Action 是临时的，当工具完成时（包括流式完成），它将被弹出并附加到工具事件中。
//
// 限制：
//   - 此函数仅供在 ChatModelAgent 运行中使用。它依赖 ChatModelAgent 的内部 State 来存储和弹出 Action，这在其他 Agent 类型中不可用。
func SendToolGenAction(ctx context.Context, toolName string, action *AgentAction) error {
	key := toolName
	toolCallID := compose.GetToolCallID(ctx)
	if len(toolCallID) > 0 {
		key = toolCallID
	}

	return compose.ProcessState(ctx, func(ctx context.Context, st *State) error {
		st.ToolGenActions[key] = action

		return nil
	})
}

func popToolGenAction(ctx context.Context, toolName string) *AgentAction {
	toolCallID := compose.GetToolCallID(ctx)

	var action *AgentAction
	err := compose.ProcessState(ctx, func(ctx context.Context, st *State) error {
		if len(toolCallID) > 0 {
			if a := st.ToolGenActions[toolCallID]; a != nil {
				action = a
				delete(st.ToolGenActions, toolCallID)
				return nil
			}
		}

		if a := st.ToolGenActions[toolName]; a != nil {
			action = a
			delete(st.ToolGenActions, toolName)
		}

		return nil
	})

	if err != nil {
		panic("impossible")
	}

	return action
}

func newAdkToolResultCollectorMiddleware() compose.ToolMiddleware {
	return compose.ToolMiddleware{
		Invokable: func(next compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
				senders := getToolResultSendersFromCtx(ctx)
				var sender adkToolResultSender
				if senders != nil {
					sender = senders.sender
				}
				output, err := next(ctx, input)
				if err != nil {
					return nil, err
				}
				prePopAction := popToolGenAction(ctx, input.Name)
				if sender != nil {
					sender(ctx, input.Name, input.CallID, output.Result, prePopAction)
				}
				return output, nil
			}
		},
		Streamable: func(next compose.StreamableToolEndpoint) compose.StreamableToolEndpoint {
			return func(ctx context.Context, input *compose.ToolInput) (*compose.StreamToolOutput, error) {
				senders := getToolResultSendersFromCtx(ctx)
				var streamSender adkStreamToolResultSender
				if senders != nil {
					streamSender = senders.streamSender
				}
				output, err := next(ctx, input)
				if err != nil {
					return nil, err
				}
				prePopAction := popToolGenAction(ctx, input.Name)
				if streamSender != nil {
					streams := output.Result.Copy(2)
					streamSender(ctx, input.Name, input.CallID, streams[0], prePopAction)
					output.Result = streams[1]
				}
				return output, nil
			}
		},
	}
}

type reactConfig struct {
	// model 是底层的 ChatModel。
	model model.ToolCallingChatModel

	// toolsConfig 是工具节点配置。
	toolsConfig *compose.ToolsNodeConfig

	// toolsReturnDirectly 指定哪些工具需要直接返回结果。
	toolsReturnDirectly map[string]bool

	// agentName 是 Agent 的名称。
	agentName string

	// maxIterations 是最大迭代次数。
	maxIterations int

	// beforeChatModel 是在 ChatModel 调用前的钩子函数。
	beforeChatModel []func(context.Context, *ChatModelAgentState) error
	// afterChatModel 是在 ChatModel 调用后的钩子函数。
	afterChatModel []func(context.Context, *ChatModelAgentState) error

	// modelRetryConfig 是模型重试配置。
	modelRetryConfig *ModelRetryConfig
}

// genToolInfos 根据工具配置生成 ToolInfo 列表。
func genToolInfos(ctx context.Context, config *compose.ToolsNodeConfig) ([]*schema.ToolInfo, error) {
	toolInfos := make([]*schema.ToolInfo, 0, len(config.Tools))
	for _, t := range config.Tools {
		tl, err := t.Info(ctx)
		if err != nil {
			return nil, err
		}

		toolInfos = append(toolInfos, tl)
	}

	return toolInfos, nil
}

type reactGraph = *compose.Graph[[]Message, Message]
type sToolNodeOutput = *schema.StreamReader[[]Message]
type sGraphOutput = MessageStream

// getReturnDirectlyToolCallID 获取触发“直接返回”的工具调用 ID。
// 检查 State 中是否设置了 ReturnDirectlyToolCallID。
func getReturnDirectlyToolCallID(ctx context.Context) (string, bool) {
	var toolCallID string
	var hasReturnDirectly bool
	handler := func(_ context.Context, st *State) error {
		toolCallID = st.ReturnDirectlyToolCallID
		hasReturnDirectly = st.HasReturnDirectly
		return nil
	}

	_ = compose.ProcessState(ctx, handler)

	return toolCallID, hasReturnDirectly
}

// newReact 创建一个新的 ReAct 代理图（Graph）。
// 它构建了一个包含 ChatModel 和 ToolNode 的循环图，用于执行推理-行动循环。
func newReact(ctx context.Context, config *reactConfig) (reactGraph, error) {
	genState := func(ctx context.Context) *State {
		return &State{
			ToolGenActions: map[string]*AgentAction{},
			AgentName:      config.agentName,
			RemainingIterations: func() int {
				if config.maxIterations <= 0 {
					return 20
				}
				return config.maxIterations
			}(),
		}
	}

	const (
		chatModel_ = "ChatModel"
		toolNode_  = "ToolNode"
	)

	g := compose.NewGraph[[]Message, Message](compose.WithGenLocalState(genState))

	toolsInfo, err := genToolInfos(ctx, config.toolsConfig)
	if err != nil {
		return nil, err
	}

	baseModel := config.model
	if config.modelRetryConfig != nil {
		baseModel = newRetryChatModel(config.model, config.modelRetryConfig)
	}

	chatModel, err := baseModel.WithTools(toolsInfo)
	if err != nil {
		return nil, err
	}

	config.toolsConfig.ToolCallMiddlewares = append(
		[]compose.ToolMiddleware{newAdkToolResultCollectorMiddleware()},
		config.toolsConfig.ToolCallMiddlewares...,
	)

	toolsNode, err := compose.NewToolNode(ctx, config.toolsConfig)
	if err != nil {
		return nil, err
	}

	modelPreHandle := func(ctx context.Context, input []Message, st *State) ([]Message, error) {
		if st.RemainingIterations <= 0 {
			return nil, ErrExceedMaxIterations
		}
		st.RemainingIterations--

		s := &ChatModelAgentState{Messages: append(st.Messages, input...)}
		for _, b := range config.beforeChatModel {
			err = b(ctx, s)
			if err != nil {
				return nil, err
			}
		}
		st.Messages = s.Messages

		return st.Messages, nil
	}
	modelPostHandle := func(ctx context.Context, input Message, st *State) (Message, error) {
		s := &ChatModelAgentState{Messages: append(st.Messages, input)}
		for _, a := range config.afterChatModel {
			err = a(ctx, s)
			if err != nil {
				return nil, err
			}
		}
		st.Messages = s.Messages
		return input, nil
	}
	_ = g.AddChatModelNode(chatModel_, chatModel,
		compose.WithStatePreHandler(modelPreHandle), compose.WithStatePostHandler(modelPostHandle), compose.WithNodeName(chatModel_))

	toolPreHandle := func(ctx context.Context, input Message, st *State) (Message, error) {
		input = st.Messages[len(st.Messages)-1]
		if len(config.toolsReturnDirectly) > 0 {
			for i := range input.ToolCalls {
				toolName := input.ToolCalls[i].Function.Name
				if config.toolsReturnDirectly[toolName] {
					st.ReturnDirectlyToolCallID = input.ToolCalls[i].ID
					st.HasReturnDirectly = true
				}
			}
		}

		return input, nil
	}

	_ = g.AddToolsNode(toolNode_, toolsNode,
		compose.WithStatePreHandler(toolPreHandle), compose.WithNodeName(toolNode_))

	_ = g.AddEdge(compose.START, chatModel_)

	toolCallCheck := func(ctx context.Context, sMsg MessageStream) (string, error) {
		defer sMsg.Close()
		for {
			chunk, err_ := sMsg.Recv()
			if err_ != nil {
				if err_ == io.EOF {
					return compose.END, nil
				}

				return "", err_
			}

			if len(chunk.ToolCalls) > 0 {
				return toolNode_, nil
			}
		}
	}
	branch := compose.NewStreamGraphBranch(toolCallCheck, map[string]bool{compose.END: true, toolNode_: true})
	_ = g.AddBranch(chatModel_, branch)

	if len(config.toolsReturnDirectly) == 0 {
		_ = g.AddEdge(toolNode_, chatModel_)
	} else {
		const (
			toolNodeToEndConverter = "ToolNodeToEndConverter"
		)

		cvt := func(ctx context.Context, sToolCallMessages sToolNodeOutput) (sGraphOutput, error) {
			id, _ := getReturnDirectlyToolCallID(ctx)

			return schema.StreamReaderWithConvert(sToolCallMessages,
				func(in []Message) (Message, error) {

					for _, chunk := range in {
						if chunk != nil && chunk.ToolCallID == id {
							return chunk, nil
						}
					}

					return nil, schema.ErrNoValue
				}), nil
		}

		_ = g.AddLambdaNode(toolNodeToEndConverter, compose.TransformableLambda(cvt),
			compose.WithNodeName(toolNodeToEndConverter))
		_ = g.AddEdge(toolNodeToEndConverter, compose.END)

		checkReturnDirect := func(ctx context.Context,
			sToolCallMessages sToolNodeOutput) (string, error) {

			_, ok := getReturnDirectlyToolCallID(ctx)

			if ok {
				return toolNodeToEndConverter, nil
			}

			return chatModel_, nil
		}

		branch = compose.NewStreamGraphBranch(checkReturnDirect,
			map[string]bool{toolNodeToEndConverter: true, chatModel_: true})
		_ = g.AddBranch(toolNode_, branch)
	}

	return g, nil
}
