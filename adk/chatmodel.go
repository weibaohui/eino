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
	"math"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/bytedance/sonic"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/internal/core"
	"github.com/cloudwego/eino/internal/safe"
	"github.com/cloudwego/eino/schema"
	ub "github.com/cloudwego/eino/utils/callbacks"
)

const (
	addrDepthChain      = 1
	addrDepthReactGraph = 2
	addrDepthChatModel  = 3
	addrDepthToolsNode  = 3
	addrDepthTool       = 4
)

// chatModelAgentRunOptions 定义了 ChatModel Agent 的运行选项。
type chatModelAgentRunOptions struct {
	// run 阶段的选项
	chatModelOptions []model.Option
	toolOptions      []tool.Option
	agentToolOptions map[ /*tool name*/ string][]AgentRunOption // todo: map or list?

	// resume 阶段的选项
	historyModifier func(context.Context, []Message) []Message
}

// WithChatModelOptions 设置底层 Chat Model 的选项。
// 为什么要做这个：允许用户自定义 Chat Model 的行为，如温度、最大 Token 数等。
// 如何使用：传入 model.Option 列表。
func WithChatModelOptions(opts []model.Option) AgentRunOption {
	return WrapImplSpecificOptFn(func(t *chatModelAgentRunOptions) {
		t.chatModelOptions = opts
	})
}

// WithToolOptions 设置 Chat Model Agent 使用的工具的选项。
// 为什么要做这个：允许用户自定义工具的行为。
// 如何使用：传入 tool.Option 列表。
func WithToolOptions(opts []tool.Option) AgentRunOption {
	return WrapImplSpecificOptFn(func(t *chatModelAgentRunOptions) {
		t.toolOptions = opts
	})
}

// WithAgentToolRunOptions 为 Agent 指定每个工具的运行选项。
// 为什么要做这个：允许对特定的 Agent 工具进行精细化配置。
// 如何使用：传入一个 map，键为工具名称，值为 AgentRunOption 列表。
func WithAgentToolRunOptions(opts map[string] /*tool name*/ []AgentRunOption) AgentRunOption {
	return WrapImplSpecificOptFn(func(t *chatModelAgentRunOptions) {
		t.agentToolOptions = opts
	})
}

// WithHistoryModifier 设置一个函数，用于在 Resume 时修改历史记录。
// Deprecated: 请使用 ResumeWithData 和 ChatModelAgentResumeData 代替。
// 为什么要做这个：允许在恢复会话时对历史消息进行处理。
func WithHistoryModifier(f func(context.Context, []Message) []Message) AgentRunOption {
	return WrapImplSpecificOptFn(func(t *chatModelAgentRunOptions) {
		t.historyModifier = f
	})
}

// ToolsConfig 定义了工具相关的配置。
type ToolsConfig struct {
	compose.ToolsNodeConfig

	// ReturnDirectly 指定哪些工具被调用时会导致 Agent 立即返回。
	// 如果同时调用了多个列出的工具，只有第一个会触发返回。
	// Map 的键是工具名称，值为布尔值（指示是否触发立即返回）。
	ReturnDirectly map[string]bool

	// EmitInternalEvents 指示是否将 agentTool 的内部事件发送到父 Agent 的 AsyncGenerator。
	// 这允许通过 Runner 将嵌套 Agent 的输出实时流式传输给最终用户。
	//
	// 注意，这些转发的事件**不会**被记录在父 Agent 的 runSession 中。
	// 它们仅被发送给最终用户，不会影响父 Agent 的状态或检查点。
	//
	// Action Scoping (动作作用域):
	// 内部 Agent 发出的动作被限制在 Agent 工具的边界内：
	//   - Interrupted: 通过 CompositeInterrupt 传播，允许跨边界正确中断/恢复
	//   - Exit, TransferToAgent, BreakLoop: 在 Agent 工具外部被忽略
	EmitInternalEvents bool
}

// GenModelInput 是一个函数类型，用于将 Agent 指令和输入转换为适合模型的格式。
type GenModelInput func(ctx context.Context, instruction string, input *AgentInput) ([]Message, error)

// defaultGenModelInput 是默认的模型输入生成函数。
// 它将指令（System Message）和输入消息组合在一起。
// 如果存在 SessionValues，它还会尝试使用 FString 模板格式化指令。
func defaultGenModelInput(ctx context.Context, instruction string, input *AgentInput) ([]Message, error) {
	msgs := make([]Message, 0, len(input.Messages)+1)

	if instruction != "" {
		sp := schema.SystemMessage(instruction)

		vs := GetSessionValues(ctx)
		if len(vs) > 0 {
			ct := prompt.FromMessages(schema.FString, sp)
			ms, err := ct.Format(ctx, vs)
			if err != nil {
				return nil, fmt.Errorf("defaultGenModelInput: failed to format instruction using FString template. "+
					"This formatting is triggered automatically when SessionValues are present. "+
					"If your instruction contains literal curly braces (e.g., JSON), provide a custom GenModelInput that uses another format. If you are using "+
					"SessionValues for purposes other than instruction formatting, provide a custom GenModelInput that does no formatting at all: %w", err)
			}

			sp = ms[0]
		}

		msgs = append(msgs, sp)
	}

	msgs = append(msgs, input.Messages...)

	return msgs, nil
}

// ChatModelAgentState 表示 ChatModel Agent 在对话期间的状态。
type ChatModelAgentState struct {
	// Messages 包含当前对话会话中的所有消息。
	Messages []Message
}

// AgentMiddleware 提供在执行的各个阶段自定义 Agent 行为的钩子。
type AgentMiddleware struct {
	// AdditionalInstruction 向 Agent 的系统指令添加补充文本。
	// 该指令会在每次 Chat Model 调用之前拼接到基础指令中。
	AdditionalInstruction string

	// AdditionalTools 向 Agent 的可用工具集添加补充工具。
	// 这些工具将与 Agent 配置的工具合并。
	AdditionalTools []tool.BaseTool

	// BeforeChatModel 在每次 ChatModel 调用之前被调用，允许修改 Agent 状态。
	BeforeChatModel func(context.Context, *ChatModelAgentState) error

	// AfterChatModel 在每次 ChatModel 调用之后被调用，允许修改 Agent 状态。
	AfterChatModel func(context.Context, *ChatModelAgentState) error

	// WrapToolCall 使用自定义中间件逻辑包装工具调用。
	// 每个中间件包含用于工具调用的 Invokable 和/或 Streamable 函数。
	WrapToolCall compose.ToolMiddleware
}

// ChatModelAgentConfig 定义了 ChatModel Agent 的配置。
type ChatModelAgentConfig struct {
	// Name Agent 的名称。最好在所有 Agent 中保持唯一。
	Name string
	// Description Agent 能力的描述。
	// 帮助其他 Agent 决定是否将任务转移给此 Agent。
	Description string
	// Instruction 用作此 Agent 的系统提示（System Prompt）。
	// 可选。如果为空，将不使用系统提示。
	// 在默认的 GenModelInput 中支持使用 f-string 占位符来引用 SessionValues，例如：
	// "You are a helpful assistant. The current time is {Time}. The current user is {User}."
	// 这些占位符将被替换为 "Time" 和 "User" 的 SessionValues。
	Instruction string

	// Model 底层的 ToolCallingChatModel。
	Model model.ToolCallingChatModel

	// ToolsConfig 工具配置。
	ToolsConfig ToolsConfig

	// GenModelInput 将指令和输入消息转换为模型输入格式的函数。
	// 可选。默认为 defaultGenModelInput，它结合了指令和消息。
	GenModelInput GenModelInput

	// Exit 定义用于终止 Agent 进程的工具。
	// 可选。如果为 nil，将不会生成 Exit Action。
	// 可以直接使用提供的 'ExitTool' 实现。
	Exit tool.BaseTool

	// OutputKey 将 Agent 的响应存储在 Session 中。
	// 可选。设置后，通过 AddSessionValue(ctx, outputKey, msg.Content) 存储输出。
	OutputKey string

	// MaxIterations 定义 ChatModel 生成周期的上限。
	// 如果超过此限制，Agent 将以错误终止。
	// 可选。默认为 20。
	MaxIterations int

	// Middlewares 配置用于扩展功能的 Agent 中间件。
	Middlewares []AgentMiddleware

	// ModelRetryConfig 配置 ChatModel 的重试行为。
	// 设置后，Agent 将根据配置的策略自动重试失败的 ChatModel 调用。
	// 可选。如果为 nil，将不执行重试。
	ModelRetryConfig *ModelRetryConfig
}

// ChatModelAgent 是基于 ChatModel 的 Agent 实现。
type ChatModelAgent struct {
	name        string
	description string
	instruction string

	model       model.ToolCallingChatModel
	toolsConfig ToolsConfig

	genModelInput GenModelInput

	outputKey     string
	maxIterations int

	subAgents   []Agent
	parentAgent Agent

	disallowTransferToParent bool

	exit tool.BaseTool

	beforeChatModels, afterChatModels []func(context.Context, *ChatModelAgentState) error

	modelRetryConfig *ModelRetryConfig

	// runner
	once   sync.Once
	run    runFunc
	frozen uint32
}

type runFunc func(ctx context.Context, input *AgentInput, generator *AsyncGenerator[*AgentEvent], store *bridgeStore, opts ...compose.Option)

// NewChatModelAgent 使用提供的配置构建一个基于 Chat Model 的 Agent。
func NewChatModelAgent(_ context.Context, config *ChatModelAgentConfig) (*ChatModelAgent, error) {
	if config.Name == "" {
		return nil, errors.New("agent 'Name' is required")
	}
	if config.Description == "" {
		return nil, errors.New("agent 'Description' is required")
	}
	if config.Model == nil {
		return nil, errors.New("agent 'Model' is required")
	}

	genInput := defaultGenModelInput
	if config.GenModelInput != nil {
		genInput = config.GenModelInput
	}

	beforeChatModels := make([]func(context.Context, *ChatModelAgentState) error, 0)
	afterChatModels := make([]func(context.Context, *ChatModelAgentState) error, 0)
	sb := &strings.Builder{}
	sb.WriteString(config.Instruction)
	tc := config.ToolsConfig
	for _, m := range config.Middlewares {
		sb.WriteString("\n")
		sb.WriteString(m.AdditionalInstruction)
		tc.Tools = append(tc.Tools, m.AdditionalTools...)

		if m.WrapToolCall.Invokable != nil || m.WrapToolCall.Streamable != nil {
			tc.ToolCallMiddlewares = append(tc.ToolCallMiddlewares, m.WrapToolCall)
		}
		if m.BeforeChatModel != nil {
			beforeChatModels = append(beforeChatModels, m.BeforeChatModel)
		}
		if m.AfterChatModel != nil {
			afterChatModels = append(afterChatModels, m.AfterChatModel)
		}
	}

	return &ChatModelAgent{
		name:             config.Name,
		description:      config.Description,
		instruction:      sb.String(),
		model:            config.Model,
		toolsConfig:      tc,
		genModelInput:    genInput,
		exit:             config.Exit,
		outputKey:        config.OutputKey,
		maxIterations:    config.MaxIterations,
		beforeChatModels: beforeChatModels,
		afterChatModels:  afterChatModels,
		modelRetryConfig: config.ModelRetryConfig,
	}, nil
}

const (
	// TransferToAgentToolName 是转接工具的名称。
	TransferToAgentToolName = "transfer_to_agent"
	// TransferToAgentToolDesc 是转接工具的描述。
	TransferToAgentToolDesc = "Transfer the question to another agent."
)

var (
	toolInfoTransferToAgent = &schema.ToolInfo{
		Name: TransferToAgentToolName,
		Desc: TransferToAgentToolDesc,

		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"agent_name": {
				Desc:     "the name of the agent to transfer to",
				Required: true,
				Type:     schema.String,
			},
		}),
	}

	ToolInfoExit = &schema.ToolInfo{
		Name: "exit",
		Desc: "Exit the agent process and return the final result.",

		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"final_result": {
				Desc:     "the final result to return",
				Required: true,
				Type:     schema.String,
			},
		}),
	}
)

type ExitTool struct{}

func (et ExitTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return ToolInfoExit, nil
}

func (et ExitTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	type exitParams struct {
		FinalResult string `json:"final_result"`
	}

	params := &exitParams{}
	err := sonic.UnmarshalString(argumentsInJSON, params)
	if err != nil {
		return "", err
	}

	err = SendToolGenAction(ctx, "exit", NewExitAction())
	if err != nil {
		return "", err
	}

	return params.FinalResult, nil
}

// transferToAgent 实现转接 Agent 的工具。
type transferToAgent struct{}

func (tta transferToAgent) Info(_ context.Context) (*schema.ToolInfo, error) {
	return toolInfoTransferToAgent, nil
}

func transferToAgentToolOutput(destName string) string {
	return fmt.Sprintf("successfully transferred to agent [%s]", destName)
}

func (tta transferToAgent) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	type transferParams struct {
		AgentName string `json:"agent_name"`
	}

	params := &transferParams{}
	err := sonic.UnmarshalString(argumentsInJSON, params)
	if err != nil {
		return "", err
	}

	err = SendToolGenAction(ctx, TransferToAgentToolName, NewTransferToAgentAction(params.AgentName))
	if err != nil {
		return "", err
	}

	return transferToAgentToolOutput(params.AgentName), nil
}

func (a *ChatModelAgent) Name(_ context.Context) string {
	return a.name
}

func (a *ChatModelAgent) Description(_ context.Context) string {
	return a.description
}

func (a *ChatModelAgent) OnSetSubAgents(_ context.Context, subAgents []Agent) error {
	if atomic.LoadUint32(&a.frozen) == 1 {
		return errors.New("agent has been frozen after run")
	}

	if len(a.subAgents) > 0 {
		return errors.New("agent's sub-agents has already been set")
	}

	a.subAgents = subAgents
	return nil
}

func (a *ChatModelAgent) OnSetAsSubAgent(_ context.Context, parent Agent) error {
	if atomic.LoadUint32(&a.frozen) == 1 {
		return errors.New("agent has been frozen after run")
	}

	if a.parentAgent != nil {
		return errors.New("agent has already been set as a sub-agent of another agent")
	}

	a.parentAgent = parent
	return nil
}

func (a *ChatModelAgent) OnDisallowTransferToParent(_ context.Context) error {
	if atomic.LoadUint32(&a.frozen) == 1 {
		return errors.New("agent has been frozen after run")
	}

	a.disallowTransferToParent = true

	return nil
}

// cbHandler 处理 ChatModel 和 ToolsNode 的回调。
type cbHandler struct {
	*AsyncGenerator[*AgentEvent]
	agentName string

	enableStreaming         bool
	store                   *bridgeStore
	returnDirectlyToolEvent atomic.Value
	ctx                     context.Context
	addr                    Address

	modelRetryConfigs *ModelRetryConfig
}

func (h *cbHandler) onChatModelEnd(ctx context.Context,
	_ *callbacks.RunInfo, output *model.CallbackOutput) context.Context {
	addr := core.GetCurrentAddress(ctx)
	if !isAddressAtDepth(addr, h.addr, addrDepthChatModel) {
		return ctx
	}

	event := EventFromMessage(output.Message, nil, schema.Assistant, "")
	h.Send(event)
	return ctx
}

func (h *cbHandler) onChatModelEndWithStreamOutput(ctx context.Context,
	_ *callbacks.RunInfo, output *schema.StreamReader[*model.CallbackOutput]) context.Context {
	addr := core.GetCurrentAddress(ctx)
	if !isAddressAtDepth(addr, h.addr, addrDepthChatModel) {
		return ctx
	}

	var convertOpts []schema.ConvertOption
	if h.modelRetryConfigs != nil {
		retryInfo, exists := getStreamRetryInfo(ctx)
		if !exists {
			retryInfo = &streamRetryInfo{attempt: 0}
		}
		convertOpts = append(convertOpts, schema.WithErrWrapper(genErrWrapper(ctx, *h.modelRetryConfigs, *retryInfo)))
	}

	cvt := func(in *model.CallbackOutput) (Message, error) {
		return in.Message, nil
	}
	out := schema.StreamReaderWithConvert(output, cvt, convertOpts...)
	event := EventFromMessage(nil, out, schema.Assistant, "")
	h.Send(event)

	return ctx
}

func (h *cbHandler) sendReturnDirectlyToolEvent() {
	if e, ok := h.returnDirectlyToolEvent.Load().(*AgentEvent); ok && e != nil {
		h.Send(e)
	}
}

func (h *cbHandler) onToolsNodeEnd(ctx context.Context, _ *callbacks.RunInfo, _ []*schema.Message) context.Context {
	addr := core.GetCurrentAddress(ctx)
	if !isAddressAtDepth(addr, h.addr, addrDepthToolsNode) {
		return ctx
	}
	h.sendReturnDirectlyToolEvent()
	return ctx
}

func (h *cbHandler) onToolsNodeEndWithStreamOutput(ctx context.Context, _ *callbacks.RunInfo, _ *schema.StreamReader[[]*schema.Message]) context.Context {
	addr := core.GetCurrentAddress(ctx)
	if !isAddressAtDepth(addr, h.addr, addrDepthToolsNode) {
		return ctx
	}
	h.sendReturnDirectlyToolEvent()
	return ctx
}

type ChatModelAgentInterruptInfo struct { // replace temp info by info when save the data
	Info *compose.InterruptInfo
	Data []byte
}

func init() {
	schema.RegisterName[*ChatModelAgentInterruptInfo]("_eino_adk_chat_model_agent_interrupt_info")
}

func (h *cbHandler) onGraphError(ctx context.Context,
	_ *callbacks.RunInfo, err error) context.Context {
	addr := core.GetCurrentAddress(ctx)
	if !isAddressAtDepth(addr, h.addr, addrDepthChain) {
		return ctx
	}

	info, ok := compose.ExtractInterruptInfo(err)
	if !ok {
		h.Send(&AgentEvent{Err: err})
		return ctx
	}

	data, existed, err := h.store.Get(ctx, bridgeCheckpointID)
	if err != nil {
		h.Send(&AgentEvent{AgentName: h.agentName, Err: fmt.Errorf("failed to get interrupt info: %w", err)})
		return ctx
	}
	if !existed {
		h.Send(&AgentEvent{AgentName: h.agentName, Err: fmt.Errorf("interrupt occurred but checkpoint data is missing")})
		return ctx
	}

	is := FromInterruptContexts(info.InterruptContexts)

	event := CompositeInterrupt(h.ctx, info, data, is)
	event.Action.Interrupted.Data = &ChatModelAgentInterruptInfo{ // for backward-compatibility with older checkpoints
		Info: info,
		Data: data,
	}
	event.AgentName = h.agentName
	h.Send(event)

	return ctx
}

func genReactCallbacks(ctx context.Context, agentName string,
	generator *AsyncGenerator[*AgentEvent],
	enableStreaming bool,
	store *bridgeStore,
	modelRetryConfigs *ModelRetryConfig) compose.Option {

	h := &cbHandler{
		ctx:               ctx,
		addr:              core.GetCurrentAddress(ctx),
		AsyncGenerator:    generator,
		agentName:         agentName,
		store:             store,
		enableStreaming:   enableStreaming,
		modelRetryConfigs: modelRetryConfigs}

	cmHandler := &ub.ModelCallbackHandler{
		OnEnd:                 h.onChatModelEnd,
		OnEndWithStreamOutput: h.onChatModelEndWithStreamOutput,
	}
	toolsNodeHandler := &ub.ToolsNodeCallbackHandlers{
		OnEnd:                 h.onToolsNodeEnd,
		OnEndWithStreamOutput: h.onToolsNodeEndWithStreamOutput,
	}
	createToolResultSender := func() adkToolResultSender {
		return func(toolCtx context.Context, toolName, callID, result string, prePopAction *AgentAction) {
			msg := schema.ToolMessage(result, callID, schema.WithToolName(toolName))
			event := EventFromMessage(msg, nil, schema.Tool, toolName)

			if prePopAction != nil {
				event.Action = prePopAction
			} else {
				event.Action = popToolGenAction(toolCtx, toolName)
			}

			returnDirectlyID, hasReturnDirectly := getReturnDirectlyToolCallID(toolCtx)
			if hasReturnDirectly && returnDirectlyID == callID {
				h.returnDirectlyToolEvent.Store(event)
			} else {
				h.Send(event)
			}
		}
	}
	createStreamToolResultSender := func() adkStreamToolResultSender {
		return func(toolCtx context.Context, toolName, callID string, resultStream *schema.StreamReader[string], prePopAction *AgentAction) {
			cvt := func(in string) (Message, error) {
				return schema.ToolMessage(in, callID, schema.WithToolName(toolName)), nil
			}
			msgStream := schema.StreamReaderWithConvert(resultStream, cvt)
			event := EventFromMessage(nil, msgStream, schema.Tool, toolName)
			event.Action = prePopAction

			returnDirectlyID, hasReturnDirectly := getReturnDirectlyToolCallID(toolCtx)
			if hasReturnDirectly && returnDirectlyID == callID {
				h.returnDirectlyToolEvent.Store(event)
			} else {
				h.Send(event)
			}
		}
	}
	reactGraphHandler := callbacks.NewHandlerBuilder().
		OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
			currentAddr := core.GetCurrentAddress(ctx)
			if !isAddressAtDepth(currentAddr, h.addr, addrDepthReactGraph) {
				return ctx
			}
			return setToolResultSendersToCtx(ctx, h.addr, createToolResultSender(), createStreamToolResultSender())
		}).
		OnStartWithStreamInputFn(func(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) context.Context {
			currentAddr := core.GetCurrentAddress(ctx)
			if !isAddressAtDepth(currentAddr, h.addr, addrDepthReactGraph) {
				return ctx
			}
			return setToolResultSendersToCtx(ctx, h.addr, createToolResultSender(), createStreamToolResultSender())
		}).Build()
	chainHandler := callbacks.NewHandlerBuilder().OnErrorFn(h.onGraphError).Build()

	cb := ub.NewHandlerHelper().ChatModel(cmHandler).ToolsNode(toolsNodeHandler).Graph(reactGraphHandler).Chain(chainHandler).Handler()

	return compose.WithCallbacks(cb)
}

type noToolsCbHandler struct {
	*AsyncGenerator[*AgentEvent]
	modelRetryConfigs *ModelRetryConfig
}

func (h *noToolsCbHandler) onChatModelEnd(ctx context.Context,
	_ *callbacks.RunInfo, output *model.CallbackOutput) context.Context {
	event := EventFromMessage(output.Message, nil, schema.Assistant, "")
	h.Send(event)
	return ctx
}

func (h *noToolsCbHandler) onChatModelEndWithStreamOutput(ctx context.Context,
	_ *callbacks.RunInfo, output *schema.StreamReader[*model.CallbackOutput]) context.Context {
	var convertOpts []schema.ConvertOption
	if h.modelRetryConfigs != nil {
		retryInfo, exists := getStreamRetryInfo(ctx)
		if !exists {
			retryInfo = &streamRetryInfo{attempt: 0}
		}
		convertOpts = append(convertOpts, schema.WithErrWrapper(genErrWrapper(ctx, *h.modelRetryConfigs, *retryInfo)))
	}

	cvt := func(in *model.CallbackOutput) (Message, error) {
		return in.Message, nil
	}
	out := schema.StreamReaderWithConvert(output, cvt, convertOpts...)
	event := EventFromMessage(nil, out, schema.Assistant, "")
	h.Send(event)
	return ctx
}

func (h *noToolsCbHandler) onGraphError(ctx context.Context,
	_ *callbacks.RunInfo, err error) context.Context {
	h.Send(&AgentEvent{Err: err})
	return ctx
}

func genNoToolsCallbacks(generator *AsyncGenerator[*AgentEvent], modelRetryConfigs *ModelRetryConfig) compose.Option {
	h := &noToolsCbHandler{
		AsyncGenerator:    generator,
		modelRetryConfigs: modelRetryConfigs,
	}

	cmHandler := &ub.ModelCallbackHandler{
		OnEnd:                 h.onChatModelEnd,
		OnEndWithStreamOutput: h.onChatModelEndWithStreamOutput,
	}
	graphHandler := callbacks.NewHandlerBuilder().OnErrorFn(h.onGraphError).Build()

	cb := ub.NewHandlerHelper().ChatModel(cmHandler).Chain(graphHandler).Handler()

	return compose.WithCallbacks(cb)
}

func setOutputToSession(ctx context.Context, msg Message, msgStream MessageStream, outputKey string) error {
	if msg != nil {
		AddSessionValue(ctx, outputKey, msg.Content)
		return nil
	}

	concatenated, err := schema.ConcatMessageStream(msgStream)
	if err != nil {
		return err
	}

	AddSessionValue(ctx, outputKey, concatenated.Content)
	return nil
}

func errFunc(err error) runFunc {
	return func(ctx context.Context, input *AgentInput, generator *AsyncGenerator[*AgentEvent], store *bridgeStore, _ ...compose.Option) {
		generator.Send(&AgentEvent{Err: err})
	}
}

// ChatModelAgentResumeData holds data that can be provided to a ChatModelAgent during a resume operation
// to modify its behavior. It is provided via the adk.ResumeWithData function.
type ChatModelAgentResumeData struct {
	// HistoryModifier is a function that can transform the agent's message history before it is sent to the model.
	// This allows for adding new information or context upon resumption.
	HistoryModifier func(ctx context.Context, history []Message) []Message
}

func (a *ChatModelAgent) buildRunFunc(ctx context.Context) runFunc {
	a.once.Do(func() {
		instruction := a.instruction
		toolsNodeConf := a.toolsConfig.ToolsNodeConfig
		returnDirectly := copyMap(a.toolsConfig.ReturnDirectly)

		transferToAgents := a.subAgents
		if a.parentAgent != nil && !a.disallowTransferToParent {
			transferToAgents = append(transferToAgents, a.parentAgent)
		}

		if len(transferToAgents) > 0 {
			transferInstruction := genTransferToAgentInstruction(ctx, transferToAgents)
			instruction = concatInstructions(instruction, transferInstruction)

			toolsNodeConf.Tools = append(toolsNodeConf.Tools, &transferToAgent{})
			returnDirectly[TransferToAgentToolName] = true
		}

		if a.exit != nil {
			toolsNodeConf.Tools = append(toolsNodeConf.Tools, a.exit)
			exitInfo, err := a.exit.Info(ctx)
			if err != nil {
				a.run = errFunc(err)
				return
			}
			returnDirectly[exitInfo.Name] = true
		}

		if len(toolsNodeConf.Tools) == 0 {
			var chatModel model.ToolCallingChatModel = a.model
			if a.modelRetryConfig != nil {
				chatModel = newRetryChatModel(a.model, a.modelRetryConfig)
			}

			a.run = func(ctx context.Context, input *AgentInput, generator *AsyncGenerator[*AgentEvent],
				store *bridgeStore, opts ...compose.Option) {
				r, err := compose.NewChain[*AgentInput, Message](compose.WithGenLocalState(func(ctx context.Context) (state *ChatModelAgentState) {
					return &ChatModelAgentState{}
				})).
					AppendLambda(compose.InvokableLambda(func(ctx context.Context, input *AgentInput) ([]Message, error) {
						messages, err := a.genModelInput(ctx, instruction, input)
						if err != nil {
							return nil, err
						}
						return messages, nil
					})).
					AppendChatModel(
						chatModel,
						compose.WithStatePreHandler(func(ctx context.Context, in []*schema.Message, state *ChatModelAgentState) ([]*schema.Message, error) {
							state.Messages = in
							for _, bc := range a.beforeChatModels {
								err := bc(ctx, state)
								if err != nil {
									return nil, err
								}
							}
							return state.Messages, nil
						}),
						compose.WithStatePostHandler(func(ctx context.Context, in *schema.Message, state *ChatModelAgentState) (*schema.Message, error) {
							state.Messages = append(state.Messages, in)
							for _, ac := range a.afterChatModels {
								err := ac(ctx, state)
								if err != nil {
									return nil, err
								}
							}
							return state.Messages[len(state.Messages)-1], nil
						}),
					).
					Compile(ctx, compose.WithGraphName(a.name),
						compose.WithCheckPointStore(store),
						compose.WithSerializer(&gobSerializer{}))
				if err != nil {
					generator.Send(&AgentEvent{Err: err})
					return
				}

				callOpt := genNoToolsCallbacks(generator, a.modelRetryConfig)
				var runOpts []compose.Option
				runOpts = append(runOpts, opts...)
				runOpts = append(runOpts, callOpt)

				var msg Message
				var msgStream MessageStream
				if input.EnableStreaming {
					msgStream, err = r.Stream(ctx, input, runOpts...)
				} else {
					msg, err = r.Invoke(ctx, input, runOpts...)
				}

				if err == nil {
					if a.outputKey != "" {
						err = setOutputToSession(ctx, msg, msgStream, a.outputKey)
						if err != nil {
							generator.Send(&AgentEvent{Err: err})
						}
					} else if msgStream != nil {
						msgStream.Close()
					}
				}

				generator.Close()
			}

			return
		}

		// react
		conf := &reactConfig{
			model:               a.model,
			toolsConfig:         &toolsNodeConf,
			toolsReturnDirectly: returnDirectly,
			agentName:           a.name,
			maxIterations:       a.maxIterations,
			beforeChatModel:     a.beforeChatModels,
			afterChatModel:      a.afterChatModels,
			modelRetryConfig:    a.modelRetryConfig,
		}

		g, err := newReact(ctx, conf)
		if err != nil {
			a.run = errFunc(err)
			return
		}

		a.run = func(ctx context.Context, input *AgentInput, generator *AsyncGenerator[*AgentEvent], store *bridgeStore,
			opts ...compose.Option) {
			var compileOptions []compose.GraphCompileOption
			compileOptions = append(compileOptions,
				compose.WithGraphName(a.name),
				compose.WithCheckPointStore(store),
				compose.WithSerializer(&gobSerializer{}),
				// ensure the graph won't exceed max steps due to max iterations
				compose.WithMaxRunSteps(math.MaxInt))

			runnable, err_ := compose.NewChain[*AgentInput, Message]().
				AppendLambda(
					compose.InvokableLambda(func(ctx context.Context, input *AgentInput) ([]Message, error) {
						return a.genModelInput(ctx, instruction, input)
					}),
				).
				AppendGraph(g, compose.WithNodeName("ReAct"), compose.WithGraphCompileOptions(compose.WithMaxRunSteps(math.MaxInt))).
				Compile(ctx, compileOptions...)
			if err_ != nil {
				generator.Send(&AgentEvent{Err: err_})
				return
			}

			callOpt := genReactCallbacks(ctx, a.name, generator, input.EnableStreaming, store, a.modelRetryConfig)
			var runOpts []compose.Option
			runOpts = append(runOpts, opts...)
			runOpts = append(runOpts, callOpt)
			if a.toolsConfig.EmitInternalEvents {
				runOpts = append(runOpts, compose.WithToolsNodeOption(compose.WithToolOption(withAgentToolEventGenerator(generator))))
			}
			if input.EnableStreaming {
				runOpts = append(runOpts, compose.WithToolsNodeOption(compose.WithToolOption(withAgentToolEnableStreaming(true))))
			}

			var msg Message
			var msgStream MessageStream
			if input.EnableStreaming {
				msgStream, err_ = runnable.Stream(ctx, input, runOpts...)
			} else {
				msg, err_ = runnable.Invoke(ctx, input, runOpts...)
			}

			if err_ == nil {
				if a.outputKey != "" {
					err_ = setOutputToSession(ctx, msg, msgStream, a.outputKey)
					if err_ != nil {
						generator.Send(&AgentEvent{Err: err_})
					}
				} else if msgStream != nil {
					msgStream.Close()
				}
			}

			generator.Close()
		}
	})

	atomic.StoreUint32(&a.frozen, 1)

	return a.run
}

func (a *ChatModelAgent) Run(ctx context.Context, input *AgentInput, opts ...AgentRunOption) *AsyncIterator[*AgentEvent] {
	run := a.buildRunFunc(ctx)

	co := getComposeOptions(opts)
	co = append(co, compose.WithCheckPointID(bridgeCheckpointID))

	iterator, generator := NewAsyncIteratorPair[*AgentEvent]()
	go func() {
		defer func() {
			panicErr := recover()
			if panicErr != nil {
				e := safe.NewPanicErr(panicErr, debug.Stack())
				generator.Send(&AgentEvent{Err: e})
			}

			generator.Close()
		}()

		run(ctx, input, generator, newBridgeStore(), co...)
	}()

	return iterator
}

func (a *ChatModelAgent) Resume(ctx context.Context, info *ResumeInfo, opts ...AgentRunOption) *AsyncIterator[*AgentEvent] {
	run := a.buildRunFunc(ctx)

	co := getComposeOptions(opts)
	co = append(co, compose.WithCheckPointID(bridgeCheckpointID))

	if info.InterruptState == nil {
		panic(fmt.Sprintf("ChatModelAgent.Resume: agent '%s' was asked to resume but has no state", a.Name(ctx)))
	}

	stateByte, ok := info.InterruptState.([]byte)
	if !ok {
		panic(fmt.Sprintf("ChatModelAgent.Resume: agent '%s' was asked to resume but has invalid interrupt state type: %T",
			a.Name(ctx), info.InterruptState))
	}

	if info.ResumeData != nil {
		resumeData, ok := info.ResumeData.(*ChatModelAgentResumeData)
		if !ok {
			panic(fmt.Sprintf("ChatModelAgent.Resume: agent '%s' was asked to resume but has invalid resume data type: %T",
				a.Name(ctx), info.ResumeData))
		}

		if resumeData.HistoryModifier != nil {
			co = append(co, compose.WithStateModifier(func(ctx context.Context, path compose.NodePath, state any) error {
				s, ok := state.(*State)
				if !ok {
					return fmt.Errorf("unexpected state type: %T, expected: %T", state, &State{})
				}
				s.Messages = resumeData.HistoryModifier(ctx, s.Messages)
				return nil
			}))
		}
	}

	iterator, generator := NewAsyncIteratorPair[*AgentEvent]()
	go func() {
		defer func() {
			panicErr := recover()
			if panicErr != nil {
				e := safe.NewPanicErr(panicErr, debug.Stack())
				generator.Send(&AgentEvent{Err: e})
			}

			generator.Close()
		}()

		run(ctx, &AgentInput{EnableStreaming: info.EnableStreaming}, generator,
			newResumeBridgeStore(stateByte), co...)
	}()

	return iterator
}

func getComposeOptions(opts []AgentRunOption) []compose.Option {
	o := GetImplSpecificOptions[chatModelAgentRunOptions](nil, opts...)
	var co []compose.Option
	if len(o.chatModelOptions) > 0 {
		co = append(co, compose.WithChatModelOption(o.chatModelOptions...))
	}
	var to []tool.Option
	if len(o.toolOptions) > 0 {
		to = append(to, o.toolOptions...)
	}
	for toolName, atos := range o.agentToolOptions {
		to = append(to, withAgentToolOptions(toolName, atos))
	}
	if len(to) > 0 {
		co = append(co, compose.WithToolsNodeOption(compose.WithToolOption(to...)))
	}
	if o.historyModifier != nil {
		co = append(co, compose.WithStateModifier(func(ctx context.Context, path compose.NodePath, state any) error {
			s, ok := state.(*State)
			if !ok {
				return fmt.Errorf("unexpected state type: %T, expected: %T", state, &State{})
			}
			s.Messages = o.historyModifier(ctx, s.Messages)
			return nil
		}))
	}
	return co
}

type gobSerializer struct{}

func (g *gobSerializer) Marshal(v any) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := gob.NewEncoder(buf).Encode(v)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (g *gobSerializer) Unmarshal(data []byte, v any) error {
	buf := bytes.NewBuffer(data)
	return gob.NewDecoder(buf).Decode(v)
}
