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

package compose

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/internal/safe"
	"github.com/cloudwego/eino/schema"
)

type toolsNodeOptions struct {
	ToolOptions []tool.Option
	ToolList    []tool.BaseTool
}

// ToolsNodeOption is the option func type for ToolsNode.
// ToolsNodeOption 是 ToolsNode 的选项函数类型。
type ToolsNodeOption func(o *toolsNodeOptions)

// WithToolOption adds tool options to the ToolsNode.
// WithToolOption 向 ToolsNode 添加工具选项。
func WithToolOption(opts ...tool.Option) ToolsNodeOption {
	return func(o *toolsNodeOptions) {
		o.ToolOptions = append(o.ToolOptions, opts...)
	}
}

// WithToolList sets the tool list for the ToolsNode.
// WithToolList 设置 ToolsNode 的工具列表。
func WithToolList(tool ...tool.BaseTool) ToolsNodeOption {
	return func(o *toolsNodeOptions) {
		o.ToolList = tool
	}
}

// ToolsNode represents a node capable of executing tools within a graph.
// The Graph Node interface is defined as follows:
//
//	Invoke(ctx context.Context, input *schema.Message, opts ...ToolsNodeOption) ([]*schema.Message, error)
//	Stream(ctx context.Context, input *schema.Message, opts ...ToolsNodeOption) (*schema.StreamReader[[]*schema.Message], error)
//
// Input: An AssistantMessage containing ToolCalls
// Output: An array of ToolMessage where the order of elements corresponds to the order of ToolCalls in the input
// ToolsNode 表示能够在图中执行工具的节点。
// 图节点接口定义如下：
//
//	Invoke(ctx context.Context, input *schema.Message, opts ...ToolsNodeOption) ([]*schema.Message, error)
//	Stream(ctx context.Context, input *schema.Message, opts ...ToolsNodeOption) (*schema.StreamReader[[]*schema.Message], error)
//
// 输入：包含 ToolCalls 的 AssistantMessage
// 输出：ToolMessage 数组，元素的顺序对应于输入中 ToolCalls 的顺序
type ToolsNode struct {
	tuple                     *toolsTuple
	unknownToolHandler        func(ctx context.Context, name, input string) (string, error)
	executeSequentially       bool
	toolArgumentsHandler      func(ctx context.Context, name, input string) (string, error)
	toolCallMiddlewares       []InvokableToolMiddleware
	streamToolCallMiddlewares []StreamableToolMiddleware
}

// ToolInput represents the input parameters for a tool call execution.
// ToolInput 表示工具调用执行的输入参数。
type ToolInput struct {
	// Name is the name of the tool to be executed.
	// Name 是要执行的工具的名称。
	Name string
	// Arguments contains the arguments for the tool call.
	// Arguments 包含工具调用的参数。
	Arguments string
	// CallID is the unique identifier for this tool call.
	// CallID 是此工具调用的唯一标识符。
	CallID string
	// CallOptions contains tool options for the execution.
	// CallOptions 包含执行的工具选项。
	CallOptions []tool.Option
}

// ToolOutput represents the result of a non-streaming tool call execution.
// ToolOutput 表示非流式工具调用执行的结果。
type ToolOutput struct {
	// Result contains the string output from the tool execution.
	// Result 包含工具执行的字符串输出。
	Result string
}

// StreamToolOutput represents the result of a streaming tool call execution.
// StreamToolOutput 表示流式工具调用执行的结果。
type StreamToolOutput struct {
	// Result is a stream reader that provides access to the tool's streaming output.
	// Result 是一个流读取器，提供对工具流式输出的访问。
	Result *schema.StreamReader[string]
}

// InvokableToolEndpoint is the function signature for non-streaming tool calls.
// InvokableToolEndpoint 是非流式工具调用的函数签名。
type InvokableToolEndpoint func(ctx context.Context, input *ToolInput) (*ToolOutput, error)

// StreamableToolEndpoint is the function signature for streaming tool calls.
// StreamableToolEndpoint 是流式工具调用的函数签名。
type StreamableToolEndpoint func(ctx context.Context, input *ToolInput) (*StreamToolOutput, error)

// InvokableToolMiddleware is a function that wraps InvokableToolEndpoint to add custom processing logic.
// It can be used to intercept, modify, or enhance tool call execution for non-streaming tools.
// InvokableToolMiddleware 是一个包装 InvokableToolEndpoint 以添加自定义处理逻辑的函数。
// 它可以用于拦截、修改或增强非流式工具的工具调用执行。
type InvokableToolMiddleware func(InvokableToolEndpoint) InvokableToolEndpoint

// StreamableToolMiddleware is a function that wraps StreamableToolEndpoint to add custom processing logic.
// It can be used to intercept, modify, or enhance tool call execution for streaming tools.
// StreamableToolMiddleware 是一个包装 StreamableToolEndpoint 以添加自定义处理逻辑的函数。
// 它可以用于拦截、修改或增强流式工具的工具调用执行。
type StreamableToolMiddleware func(StreamableToolEndpoint) StreamableToolEndpoint

// ToolMiddleware groups middleware hooks for invokable and streamable tool calls.
// ToolMiddleware 对可调用和可流式传输的工具调用的中间件挂钩进行分组。
type ToolMiddleware struct {
	// Invokable contains middleware function for non-streaming tool calls.
	// Note: This middleware only applies to tools that implement the InvokableTool interface.
	// Invokable 包含非流式工具调用的中间件函数。
	// 注意：此中间件仅适用于实现 InvokableTool 接口的工具。
	Invokable InvokableToolMiddleware

	// Streamable contains middleware function for streaming tool calls.
	// Note: This middleware only applies to tools that implement the StreamableTool interface.
	// Streamable 包含流式工具调用的中间件函数。
	// 注意：此中间件仅适用于实现 StreamableTool 接口的工具。
	Streamable StreamableToolMiddleware
}

// ToolsNodeConfig is the config for ToolsNode.
// ToolsNodeConfig 是 ToolsNode 的配置。
type ToolsNodeConfig struct {
	// Tools specify the list of tools can be called which are BaseTool but must implement InvokableTool or StreamableTool.
	// Tools 指定可以调用的工具列表，这些工具是 BaseTool，但必须实现 InvokableTool 或 StreamableTool。
	Tools []tool.BaseTool

	// UnknownToolsHandler handles tool calls for non-existent tools when LLM hallucinates.
	// This field is optional. When not set, calling a non-existent tool will result in an error.
	// When provided, if the LLM attempts to call a tool that doesn't exist in the Tools list,
	// this handler will be invoked instead of returning an error, allowing graceful handling of hallucinated tools.
	// UnknownToolsHandler 处理 LLM 产生幻觉时对不存在工具的工具调用。
	// 此字段是可选的。如果未设置，调用不存在的工具将导致错误。
	// 如果提供，当 LLM 尝试调用 Tools 列表中不存在的工具时，
	// 将调用此处理程序而不是返回错误，从而允许优雅地处理幻觉工具。
	// Parameters:
	//   - ctx: The context for the tool call
	//   - name: The name of the non-existent tool
	//   - input: The tool call input generated by llm
	// Returns:
	//   - string: The response to be returned as if the tool was executed
	//   - error: Any error that occurred during handling
	UnknownToolsHandler func(ctx context.Context, name, input string) (string, error)

	// ExecuteSequentially determines whether tool calls should be executed sequentially (in order) or in parallel.
	// When set to true, tool calls will be executed one after another in the order they appear in the input message.
	// When set to false (default), tool calls will be executed in parallel.
	// ExecuteSequentially 确定工具调用是顺序执行（按顺序）还是并行执行。
	// 当设置为 true 时，工具调用将按照它们在输入消息中出现的顺序一个接一个地执行。
	// 当设置为 false（默认值）时，工具调用将并行执行。
	ExecuteSequentially bool

	// ToolArgumentsHandler allows handling of tool arguments before execution.
	// When provided, this function will be called for each tool call to process the arguments.
	// ToolArgumentsHandler 允许在执行前处理工具参数。
	// 如果提供，将为每个工具调用调用此函数以处理参数。
	// Parameters:
	//   - ctx: The context for the tool call
	//   - name: The name of the tool being called
	//   - arguments: The original arguments string for the tool
	// Returns:
	//   - string: The processed arguments string to be used for tool execution
	//   - error: Any error that occurred during preprocessing
	ToolArgumentsHandler func(ctx context.Context, name, arguments string) (string, error)

	// ToolCallMiddlewares configures middleware for tool calls.
	// Each element can contain Invokable and/or Streamable middleware.
	// Invokable middleware only applies to tools implementing InvokableTool interface.
	// Streamable middleware only applies to tools implementing StreamableTool interface.
	// ToolCallMiddlewares 配置工具调用的中间件。
	// 每个元素可以包含 Invokable 和/或 Streamable 中间件。
	// Invokable 中间件仅适用于实现 InvokableTool 接口的工具。
	// Streamable 中间件仅适用于实现 StreamableTool 接口的工具。
	ToolCallMiddlewares []ToolMiddleware
}

// NewToolNode creates a new ToolsNode.
// NewToolNode 创建一个新的 ToolsNode。
// e.g.
//
//	conf := &ToolsNodeConfig{
//		Tools: []tool.BaseTool{invokableTool1, streamableTool2},
//	}
//	toolsNode, err := NewToolNode(ctx, conf)
func NewToolNode(ctx context.Context, conf *ToolsNodeConfig) (*ToolsNode, error) {
	var middlewares []InvokableToolMiddleware
	var streamMiddlewares []StreamableToolMiddleware
	for _, m := range conf.ToolCallMiddlewares {
		if m.Invokable != nil {
			middlewares = append(middlewares, m.Invokable)
		}
		if m.Streamable != nil {
			streamMiddlewares = append(streamMiddlewares, m.Streamable)
		}
	}

	tuple, err := convTools(ctx, conf.Tools, middlewares, streamMiddlewares)
	if err != nil {
		return nil, err
	}

	return &ToolsNode{
		tuple:                     tuple,
		unknownToolHandler:        conf.UnknownToolsHandler,
		executeSequentially:       conf.ExecuteSequentially,
		toolArgumentsHandler:      conf.ToolArgumentsHandler,
		toolCallMiddlewares:       middlewares,
		streamToolCallMiddlewares: streamMiddlewares,
	}, nil
}

// ToolsInterruptAndRerunExtra carries interrupt metadata for ToolsNode reruns.
// ToolsInterruptAndRerunExtra 携带 ToolsNode 重运行的中断元数据。
type ToolsInterruptAndRerunExtra struct {
	ToolCalls     []schema.ToolCall
	ExecutedTools map[string]string
	RerunTools    []string
	RerunExtraMap map[string]any
}

func init() {
	schema.RegisterName[*ToolsInterruptAndRerunExtra]("_eino_compose_tools_interrupt_and_rerun_extra")
	schema.RegisterName[*toolsInterruptAndRerunState]("_eino_compose_tools_interrupt_and_rerun_state")
}

// toolsInterruptAndRerunState stores state for interrupt and rerun.
// toolsInterruptAndRerunState 存储中断和重运行的状态。
type toolsInterruptAndRerunState struct {
	Input         *schema.Message
	ExecutedTools map[string]string
	RerunTools    []string
}

// toolsTuple holds processed tool information.
// toolsTuple 保存处理后的工具信息。
type toolsTuple struct {
	indexes         map[string]int
	meta            []*executorMeta
	endpoints       []InvokableToolEndpoint
	streamEndpoints []StreamableToolEndpoint
}

// convTools converts base tools to internal tool representation.
// convTools 将基础工具转换为内部工具表示。
func convTools(ctx context.Context, tools []tool.BaseTool, ms []InvokableToolMiddleware, sms []StreamableToolMiddleware) (*toolsTuple, error) {
	ret := &toolsTuple{
		indexes:         make(map[string]int),
		meta:            make([]*executorMeta, len(tools)),
		endpoints:       make([]InvokableToolEndpoint, len(tools)),
		streamEndpoints: make([]StreamableToolEndpoint, len(tools)),
	}
	for idx, bt := range tools {
		tl, err := bt.Info(ctx)
		if err != nil {
			return nil, fmt.Errorf("(NewToolNode) failed to get tool info at idx= %d: %w", idx, err)
		}

		toolName := tl.Name
		var (
			st tool.StreamableTool
			it tool.InvokableTool

			invokable  InvokableToolEndpoint
			streamable StreamableToolEndpoint

			ok   bool
			meta *executorMeta
		)

		meta = parseExecutorInfoFromComponent(components.ComponentOfTool, bt)

		if st, ok = bt.(tool.StreamableTool); ok {
			streamable = wrapStreamToolCall(st, sms, !meta.isComponentCallbackEnabled)
		}

		if it, ok = bt.(tool.InvokableTool); ok {
			invokable = wrapToolCall(it, ms, !meta.isComponentCallbackEnabled)
		}

		if st == nil && it == nil {
			return nil, fmt.Errorf("tool %s is not invokable or streamable", toolName)
		}

		if streamable == nil {
			streamable = invokableToStreamable(invokable)
		}
		if invokable == nil {
			invokable = streamableToInvokable(streamable)
		}

		ret.indexes[toolName] = idx
		ret.meta[idx] = meta
		ret.endpoints[idx] = invokable
		ret.streamEndpoints[idx] = streamable
	}
	return ret, nil
}

// wrapToolCall wraps an invokable tool with middlewares and callbacks.
// wrapToolCall 使用中间件和回调包装可调用工具。
func wrapToolCall(it tool.InvokableTool, middlewares []InvokableToolMiddleware, needCallback bool) InvokableToolEndpoint {
	middleware := func(next InvokableToolEndpoint) InvokableToolEndpoint {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
	if needCallback {
		it = &invokableToolWithCallback{it: it}
	}
	return middleware(func(ctx context.Context, input *ToolInput) (*ToolOutput, error) {
		result, err := it.InvokableRun(ctx, input.Arguments, input.CallOptions...)
		if err != nil {
			return nil, err
		}
		return &ToolOutput{Result: result}, nil
	})
}

// wrapStreamToolCall wraps a streamable tool with middlewares and callbacks.
// wrapStreamToolCall 使用中间件和回调包装可流式传输工具。
func wrapStreamToolCall(st tool.StreamableTool, middlewares []StreamableToolMiddleware, needCallback bool) StreamableToolEndpoint {
	middleware := func(next StreamableToolEndpoint) StreamableToolEndpoint {
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
	if needCallback {
		st = &streamableToolWithCallback{st: st}
	}
	return middleware(func(ctx context.Context, input *ToolInput) (*StreamToolOutput, error) {
		result, err := st.StreamableRun(ctx, input.Arguments, input.CallOptions...)
		if err != nil {
			return nil, err
		}
		return &StreamToolOutput{Result: result}, nil
	})
}

// invokableToolWithCallback wraps InvokableTool to add callbacks.
// invokableToolWithCallback 包装 InvokableTool 以添加回调。
type invokableToolWithCallback struct {
	it tool.InvokableTool
}

func (i *invokableToolWithCallback) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return i.it.Info(ctx)
}

func (i *invokableToolWithCallback) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	return invokeWithCallbacks(i.it.InvokableRun)(ctx, argumentsInJSON, opts...)
}

// streamableToolWithCallback wraps StreamableTool to add callbacks.
// streamableToolWithCallback 包装 StreamableTool 以添加回调。
type streamableToolWithCallback struct {
	st tool.StreamableTool
}

func (s *streamableToolWithCallback) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return s.st.Info(ctx)
}

func (s *streamableToolWithCallback) StreamableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (*schema.StreamReader[string], error) {
	return streamWithCallbacks(s.st.StreamableRun)(ctx, argumentsInJSON, opts...)
}

func streamableToInvokable(e StreamableToolEndpoint) InvokableToolEndpoint {
	return func(ctx context.Context, input *ToolInput) (*ToolOutput, error) {
		so, err := e(ctx, input)
		if err != nil {
			return nil, err
		}
		o, err := concatStreamReader(so.Result)
		if err != nil {
			return nil, fmt.Errorf("failed to concat StreamableTool output message stream: %w", err)
		}
		return &ToolOutput{Result: o}, nil
	}
}

func invokableToStreamable(e InvokableToolEndpoint) StreamableToolEndpoint {
	return func(ctx context.Context, input *ToolInput) (*StreamToolOutput, error) {
		o, err := e(ctx, input)
		if err != nil {
			return nil, err
		}
		return &StreamToolOutput{Result: schema.StreamReaderFromArray([]string{o.Result})}, nil
	}
}

type toolCallTask struct {
	// in
	endpoint       InvokableToolEndpoint
	streamEndpoint StreamableToolEndpoint
	meta           *executorMeta
	name           string
	arg            string
	callID         string

	// out
	executed bool
	output   string
	sOutput  *schema.StreamReader[string]
	err      error
}

func (tn *ToolsNode) genToolCallTasks(ctx context.Context, tuple *toolsTuple,
	input *schema.Message, executedTools map[string]string, isStream bool) ([]toolCallTask, error) {

	if input.Role != schema.Assistant {
		return nil, fmt.Errorf("expected message role is Assistant, got %s", input.Role)
	}

	n := len(input.ToolCalls)
	if n == 0 {
		return nil, errors.New("no tool call found in input message")
	}

	toolCallTasks := make([]toolCallTask, n)

	for i := 0; i < n; i++ {
		toolCall := input.ToolCalls[i]
		if result, executed := executedTools[toolCall.ID]; executed {
			toolCallTasks[i].name = toolCall.Function.Name
			toolCallTasks[i].arg = toolCall.Function.Arguments
			toolCallTasks[i].callID = toolCall.ID
			toolCallTasks[i].executed = true
			if isStream {
				toolCallTasks[i].sOutput = schema.StreamReaderFromArray([]string{result})
			} else {
				toolCallTasks[i].output = result
			}
			continue
		}
		index, ok := tuple.indexes[toolCall.Function.Name]
		if !ok {
			if tn.unknownToolHandler == nil {
				return nil, fmt.Errorf("tool %s not found in toolsNode indexes", toolCall.Function.Name)
			}
			toolCallTasks[i] = newUnknownToolTask(toolCall.Function.Name, toolCall.Function.Arguments, toolCall.ID, tn.unknownToolHandler)
		} else {
			toolCallTasks[i].endpoint = tuple.endpoints[index]
			toolCallTasks[i].streamEndpoint = tuple.streamEndpoints[index]
			toolCallTasks[i].meta = tuple.meta[index]
			toolCallTasks[i].name = toolCall.Function.Name
			toolCallTasks[i].callID = toolCall.ID
			if tn.toolArgumentsHandler != nil {
				arg, err := tn.toolArgumentsHandler(ctx, toolCall.Function.Name, toolCall.Function.Arguments)
				if err != nil {
					return nil, fmt.Errorf("failed to executed tool[name:%s arguments:%s] arguments handler: %w", toolCall.Function.Name, toolCall.Function.Arguments, err)
				}
				toolCallTasks[i].arg = arg
			} else {
				toolCallTasks[i].arg = toolCall.Function.Arguments
			}
		}
	}

	return toolCallTasks, nil
}

// newUnknownToolTask creates a task for an unknown tool.
// newUnknownToolTask 为未知工具创建一个任务。
func newUnknownToolTask(name, arg, callID string, unknownToolHandler func(ctx context.Context, name, input string) (string, error)) toolCallTask {
	endpoint := func(ctx context.Context, input *ToolInput) (*ToolOutput, error) {
		result, err := unknownToolHandler(ctx, input.Name, input.Arguments)
		if err != nil {
			return nil, err
		}
		return &ToolOutput{
			Result: result,
		}, nil
	}
	return toolCallTask{
		endpoint:       endpoint,
		streamEndpoint: invokableToStreamable(endpoint),
		meta: &executorMeta{
			component:                  components.ComponentOfTool,
			isComponentCallbackEnabled: false,
			componentImplType:          "UnknownTool",
		},
		name:   name,
		arg:    arg,
		callID: callID,
	}
}

// runToolCallTaskByInvoke executes a tool call task using the invokable endpoint.
// runToolCallTaskByInvoke 使用可调用端点执行工具调用任务。
func runToolCallTaskByInvoke(ctx context.Context, task *toolCallTask, opts ...tool.Option) {
	if task.executed {
		return
	}
	ctx = callbacks.ReuseHandlers(ctx, &callbacks.RunInfo{
		Name:      task.name,
		Type:      task.meta.componentImplType,
		Component: task.meta.component,
	})

	ctx = setToolCallInfo(ctx, &toolCallInfo{toolCallID: task.callID})
	ctx = appendToolAddressSegment(ctx, task.name, task.callID)
	output, err := task.endpoint(ctx, &ToolInput{
		Name:        task.name,
		Arguments:   task.arg,
		CallID:      task.callID,
		CallOptions: opts,
	})
	if err != nil {
		task.err = err
	} else {
		task.output = output.Result
		task.executed = true
	}
}

// runToolCallTaskByStream executes a tool call task using the streamable endpoint.
// runToolCallTaskByStream 使用可流式传输端点执行工具调用任务。
func runToolCallTaskByStream(ctx context.Context, task *toolCallTask, opts ...tool.Option) {
	ctx = callbacks.ReuseHandlers(ctx, &callbacks.RunInfo{
		Name:      task.name,
		Type:      task.meta.componentImplType,
		Component: task.meta.component,
	})

	ctx = setToolCallInfo(ctx, &toolCallInfo{toolCallID: task.callID})
	ctx = appendToolAddressSegment(ctx, task.name, task.callID)
	output, err := task.streamEndpoint(ctx, &ToolInput{
		Name:        task.name,
		Arguments:   task.arg,
		CallID:      task.callID,
		CallOptions: opts,
	})
	if err != nil {
		task.err = err
	} else {
		task.sOutput = output.Result
		task.executed = true
	}
}

func sequentialRunToolCall(ctx context.Context,
	run func(ctx2 context.Context, callTask *toolCallTask, opts ...tool.Option),
	tasks []toolCallTask, opts ...tool.Option) {

	for i := range tasks {
		if tasks[i].executed {
			continue
		}
		run(ctx, &tasks[i], opts...)
	}
}

func parallelRunToolCall(ctx context.Context,
	run func(ctx2 context.Context, callTask *toolCallTask, opts ...tool.Option),
	tasks []toolCallTask, opts ...tool.Option) {

	if len(tasks) == 1 {
		run(ctx, &tasks[0], opts...)
		return
	}

	var wg sync.WaitGroup
	for i := 1; i < len(tasks); i++ {
		if tasks[i].executed {
			continue
		}
		wg.Add(1)
		go func(ctx_ context.Context, t *toolCallTask, opts ...tool.Option) {
			defer wg.Done()
			defer func() {
				panicErr := recover()
				if panicErr != nil {
					t.err = safe.NewPanicErr(panicErr, debug.Stack())
				}
			}()
			run(ctx_, t, opts...)
		}(ctx, &tasks[i], opts...)
	}

	if !tasks[0].executed {
		run(ctx, &tasks[0], opts...)
	}

	wg.Wait()
}

// Invoke calls the tools and collects the results of invokable tools.
// it's parallel if there are multiple tool calls in the input message.
func (tn *ToolsNode) Invoke(ctx context.Context, input *schema.Message,
	opts ...ToolsNodeOption) ([]*schema.Message, error) {

	opt := getToolsNodeOptions(opts...)
	tuple := tn.tuple
	if opt.ToolList != nil {
		var err error
		tuple, err = convTools(ctx, opt.ToolList, tn.toolCallMiddlewares, tn.streamToolCallMiddlewares)
		if err != nil {
			return nil, fmt.Errorf("failed to convert tool list from call option: %w", err)
		}
	}

	var executedTools map[string]string
	if wasInterrupted, hasState, tnState := GetInterruptState[*toolsInterruptAndRerunState](ctx); wasInterrupted && hasState {
		input = tnState.Input
		if tnState.ExecutedTools != nil {
			executedTools = tnState.ExecutedTools
		}
	}

	tasks, err := tn.genToolCallTasks(ctx, tuple, input, executedTools, false)
	if err != nil {
		return nil, err
	}

	if tn.executeSequentially {
		sequentialRunToolCall(ctx, runToolCallTaskByInvoke, tasks, opt.ToolOptions...)
	} else {
		parallelRunToolCall(ctx, runToolCallTaskByInvoke, tasks, opt.ToolOptions...)
	}

	n := len(tasks)
	output := make([]*schema.Message, n)

	rerunExtra := &ToolsInterruptAndRerunExtra{
		ToolCalls:     input.ToolCalls,
		ExecutedTools: make(map[string]string),
		RerunExtraMap: make(map[string]any),
	}
	rerunState := &toolsInterruptAndRerunState{
		Input:         input,
		ExecutedTools: make(map[string]string),
	}

	var errs []error
	for i := 0; i < n; i++ {
		if tasks[i].err != nil {
			info, ok := IsInterruptRerunError(tasks[i].err)
			if !ok {
				return nil, fmt.Errorf("failed to invoke tool[name:%s id:%s]: %w", tasks[i].name, tasks[i].callID, tasks[i].err)
			}

			rerunExtra.RerunTools = append(rerunExtra.RerunTools, tasks[i].callID)
			rerunState.RerunTools = append(rerunState.RerunTools, tasks[i].callID)
			if info != nil {
				rerunExtra.RerunExtraMap[tasks[i].callID] = info
			}

			iErr := WrapInterruptAndRerunIfNeeded(ctx,
				AddressSegment{ID: tasks[i].callID, Type: AddressSegmentTool}, tasks[i].err)
			errs = append(errs, iErr)
			continue
		}
		if tasks[i].executed {
			rerunExtra.ExecutedTools[tasks[i].callID] = tasks[i].output
			rerunState.ExecutedTools[tasks[i].callID] = tasks[i].output
		}
		if len(errs) == 0 {
			output[i] = schema.ToolMessage(tasks[i].output, tasks[i].callID, schema.WithToolName(tasks[i].name))
		}
	}
	if len(errs) > 0 {
		return nil, CompositeInterrupt(ctx, rerunExtra, rerunState, errs...)
	}

	return output, nil
}

// Stream calls the tools and collects the results of stream readers.
// it's parallel if there are multiple tool calls in the input message.
func (tn *ToolsNode) Stream(ctx context.Context, input *schema.Message,
	opts ...ToolsNodeOption) (*schema.StreamReader[[]*schema.Message], error) {

	opt := getToolsNodeOptions(opts...)
	tuple := tn.tuple
	if opt.ToolList != nil {
		var err error
		tuple, err = convTools(ctx, opt.ToolList, tn.toolCallMiddlewares, tn.streamToolCallMiddlewares)
		if err != nil {
			return nil, fmt.Errorf("failed to convert tool list from call option: %w", err)
		}
	}

	var executedTools map[string]string
	if wasInterrupted, hasState, tnState := GetInterruptState[*toolsInterruptAndRerunState](ctx); wasInterrupted && hasState {
		input = tnState.Input
		if tnState.ExecutedTools != nil {
			executedTools = tnState.ExecutedTools
		}
	}

	tasks, err := tn.genToolCallTasks(ctx, tuple, input, executedTools, true)
	if err != nil {
		return nil, err
	}

	if tn.executeSequentially {
		sequentialRunToolCall(ctx, runToolCallTaskByStream, tasks, opt.ToolOptions...)
	} else {
		parallelRunToolCall(ctx, runToolCallTaskByStream, tasks, opt.ToolOptions...)
	}

	n := len(tasks)

	rerunExtra := &ToolsInterruptAndRerunExtra{
		ToolCalls:     input.ToolCalls,
		ExecutedTools: make(map[string]string),
		RerunExtraMap: make(map[string]any),
	}
	rerunState := &toolsInterruptAndRerunState{
		Input:         input,
		ExecutedTools: make(map[string]string),
	}
	var errs []error
	// check rerun
	for i := 0; i < n; i++ {
		if tasks[i].err != nil {
			info, ok := IsInterruptRerunError(tasks[i].err)
			if !ok {
				return nil, fmt.Errorf("failed to stream tool call %s: %w", tasks[i].callID, tasks[i].err)
			}

			rerunExtra.RerunTools = append(rerunExtra.RerunTools, tasks[i].callID)
			rerunState.RerunTools = append(rerunState.RerunTools, tasks[i].callID)
			if info != nil {
				rerunExtra.RerunExtraMap[tasks[i].callID] = info
			}
			iErr := WrapInterruptAndRerunIfNeeded(ctx,
				AddressSegment{ID: tasks[i].callID, Type: AddressSegmentTool}, tasks[i].err)
			errs = append(errs, iErr)
			continue
		}
	}

	if len(errs) > 0 {
		// concat and save tool output
		for _, t := range tasks {
			if t.executed {
				o, err_ := concatStreamReader(t.sOutput)
				if err_ != nil {
					return nil, fmt.Errorf("failed to concat tool[name:%s id:%s]'s stream output: %w", t.name, t.callID, err_)
				}
				rerunExtra.ExecutedTools[t.callID] = o
				rerunState.ExecutedTools[t.callID] = o
			}
		}
		return nil, CompositeInterrupt(ctx, rerunExtra, rerunState, errs...)
	}

	// common return
	sOutput := make([]*schema.StreamReader[[]*schema.Message], n)
	for i := 0; i < n; i++ {
		index := i
		callID := tasks[i].callID
		callName := tasks[i].name
		cvt := func(s string) ([]*schema.Message, error) {
			ret := make([]*schema.Message, n)
			ret[index] = schema.ToolMessage(s, callID, schema.WithToolName(callName))

			return ret, nil
		}

		sOutput[i] = schema.StreamReaderWithConvert(tasks[i].sOutput, cvt)
	}
	return schema.MergeStreamReaders(sOutput), nil
}

// GetType returns the component type string for the Tools node.
func (tn *ToolsNode) GetType() string {
	return ""
}

func getToolsNodeOptions(opts ...ToolsNodeOption) *toolsNodeOptions {
	o := &toolsNodeOptions{
		ToolOptions: make([]tool.Option, 0),
	}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

type toolCallInfoKey struct{}
type toolCallInfo struct {
	toolCallID string
}

func setToolCallInfo(ctx context.Context, toolCallInfo *toolCallInfo) context.Context {
	return context.WithValue(ctx, toolCallInfoKey{}, toolCallInfo)
}

// GetToolCallID gets the current tool call id from the context.
func GetToolCallID(ctx context.Context) string {
	v := ctx.Value(toolCallInfoKey{})
	if v == nil {
		return ""
	}

	info, ok := v.(*toolCallInfo)
	if !ok {
		return ""
	}

	return info.toolCallID
}
