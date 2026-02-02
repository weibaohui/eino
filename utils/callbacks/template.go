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

// Package callbacks provides ready-to-use callback handler templates for components.
package callbacks

import (
	"context"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// NewHandlerHelper creates a new component template handler builder.
// This builder can be used to configure and build a component template handler,
// which can handle callback events for different components with its own struct definition,
// and fallbackTemplate can be used to handle scenarios where none of the cases are hit as a fallback.
// NewHandlerHelper 创建一个新的组件模版回调处理器构建器。
// 该构建器可用于配置和构建组件模版回调处理器，
// 该处理器可以处理不同组件的回调事件，并具有自己的结构定义，
// fallbackTemplate 可用于处理未命中任何情况的场景作为回退。
func NewHandlerHelper() *HandlerHelper {
	return &HandlerHelper{
		composeTemplates: map[components.Component]callbacks.Handler{},
	}
}

// HandlerHelper is a builder for creating a callbacks.Handler with specific handlers for different component types.
// create a handler with callbacks.NewHandlerHelper().
// eg.
//
//	helper := template.NewHandlerHelper().
//		ChatModel(&model.IndexerCallbackHandler{}).
//		Prompt(&prompt.IndexerCallbackHandler{}).
//		Handler()
//
// then use the handler with runnable.Invoke(ctx, input, compose.WithCallbacks(handler))
// HandlerHelper 是用于为不同组件类型创建特定处理器的 callbacks.Handler 构建器。
// 使用 callbacks.NewHandlerHelper() 创建处理器。
// 例如：
//
//	helper := template.NewHandlerHelper().
//		ChatModel(&model.IndexerCallbackHandler{}).
//		Prompt(&prompt.IndexerCallbackHandler{}).
//		Handler()
//
// 然后在 runnable.Invoke(ctx, input, compose.WithCallbacks(handler)) 中使用该处理器。
type HandlerHelper struct {
	promptHandler      *PromptCallbackHandler
	chatModelHandler   *ModelCallbackHandler
	embeddingHandler   *EmbeddingCallbackHandler
	indexerHandler     *IndexerCallbackHandler
	retrieverHandler   *RetrieverCallbackHandler
	loaderHandler      *LoaderCallbackHandler
	transformerHandler *TransformerCallbackHandler
	toolHandler        *ToolCallbackHandler
	toolsNodeHandler   *ToolsNodeCallbackHandlers
	composeTemplates   map[components.Component]callbacks.Handler
}

// Handler returns the callbacks.Handler created by HandlerHelper.
// Handler 返回由 HandlerHelper 创建的 callbacks.Handler。
func (c *HandlerHelper) Handler() callbacks.Handler {
	return &handlerTemplate{c}
}

// Prompt sets the prompt handler for the handler helper, which will be called when the prompt component is executed.
// Prompt 设置 HandlerHelper 的 Prompt 处理器，该处理器将在 Prompt 组件执行时被调用。
func (c *HandlerHelper) Prompt(handler *PromptCallbackHandler) *HandlerHelper {
	c.promptHandler = handler
	return c
}

// ChatModel sets the chat model handler for the handler helper, which will be called when the chat model component is executed.
// ChatModel 设置 HandlerHelper 的 ChatModel 处理器，该处理器将在 ChatModel 组件执行时被调用。
func (c *HandlerHelper) ChatModel(handler *ModelCallbackHandler) *HandlerHelper {
	c.chatModelHandler = handler
	return c
}

// Embedding sets the embedding handler for the handler helper, which will be called when the embedding component is executed.
// Embedding 设置 HandlerHelper 的 Embedding 处理器，该处理器将在 Embedding 组件执行时被调用。
func (c *HandlerHelper) Embedding(handler *EmbeddingCallbackHandler) *HandlerHelper {
	c.embeddingHandler = handler
	return c
}

// Indexer sets the indexer handler for the handler helper, which will be called when the indexer component is executed.
// Indexer 设置 HandlerHelper 的 Indexer 处理器，该处理器将在 Indexer 组件执行时被调用。
func (c *HandlerHelper) Indexer(handler *IndexerCallbackHandler) *HandlerHelper {
	c.indexerHandler = handler
	return c
}

// Retriever sets the retriever handler for the handler helper, which will be called when the retriever component is executed.
// Retriever 设置 HandlerHelper 的 Retriever 处理器，该处理器将在 Retriever 组件执行时被调用。
func (c *HandlerHelper) Retriever(handler *RetrieverCallbackHandler) *HandlerHelper {
	c.retrieverHandler = handler
	return c
}

// Loader sets the loader handler for the handler helper, which will be called when the loader component is executed.
// Loader 设置 HandlerHelper 的 Loader 处理器，该处理器将在 Loader 组件执行时被调用。
func (c *HandlerHelper) Loader(handler *LoaderCallbackHandler) *HandlerHelper {
	c.loaderHandler = handler
	return c
}

// Transformer sets the transformer handler for the handler helper, which will be called when the transformer component is executed.
// Transformer 设置 HandlerHelper 的 Transformer 处理器，该处理器将在 Transformer 组件执行时被调用。
func (c *HandlerHelper) Transformer(handler *TransformerCallbackHandler) *HandlerHelper {
	c.transformerHandler = handler
	return c
}

// Tool sets the tool handler for the handler helper, which will be called when the tool component is executed.
// Tool 设置 HandlerHelper 的 Tool 处理器，该处理器将在 Tool 组件执行时被调用。
func (c *HandlerHelper) Tool(handler *ToolCallbackHandler) *HandlerHelper {
	c.toolHandler = handler
	return c
}

// ToolsNode sets the tools node handler for the handler helper, which will be called when the tools node is executed.
// ToolsNode 设置 HandlerHelper 的 ToolsNode 处理器，该处理器将在 ToolsNode 执行时被调用。
func (c *HandlerHelper) ToolsNode(handler *ToolsNodeCallbackHandlers) *HandlerHelper {
	c.toolsNodeHandler = handler
	return c
}

// Graph sets the graph handler for the handler helper, which will be called when the graph is executed.
// Graph 设置 HandlerHelper 的 Graph 处理器，该处理器将在 Graph 执行时被调用。
func (c *HandlerHelper) Graph(handler callbacks.Handler) *HandlerHelper {
	c.composeTemplates[compose.ComponentOfGraph] = handler
	return c
}

// Chain sets the chain handler for the handler helper, which will be called when the chain is executed.
// Chain 设置 HandlerHelper 的 Chain 处理器，该处理器将在 Chain 执行时被调用。
func (c *HandlerHelper) Chain(handler callbacks.Handler) *HandlerHelper {
	c.composeTemplates[compose.ComponentOfChain] = handler
	return c
}

// Lambda sets the lambda handler for the handler helper, which will be called when the lambda is executed.
// Lambda 设置 HandlerHelper 的 Lambda 处理器，该处理器将在 Lambda 执行时被调用。
func (c *HandlerHelper) Lambda(handler callbacks.Handler) *HandlerHelper {
	c.composeTemplates[compose.ComponentOfLambda] = handler
	return c
}

type handlerTemplate struct {
	*HandlerHelper
}

// OnStart is the callback function for the start event of a component.
// implement the callbacks Handler interface.
// OnStart 是组件开始事件的回调函数。
// 实现 callbacks Handler 接口。
func (c *handlerTemplate) OnStart(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
	switch info.Component {
	case components.ComponentOfPrompt:
		return c.promptHandler.OnStart(ctx, info, prompt.ConvCallbackInput(input))
	case components.ComponentOfChatModel:
		return c.chatModelHandler.OnStart(ctx, info, model.ConvCallbackInput(input))
	case components.ComponentOfEmbedding:
		return c.embeddingHandler.OnStart(ctx, info, embedding.ConvCallbackInput(input))
	case components.ComponentOfIndexer:
		return c.indexerHandler.OnStart(ctx, info, indexer.ConvCallbackInput(input))
	case components.ComponentOfRetriever:
		return c.retrieverHandler.OnStart(ctx, info, retriever.ConvCallbackInput(input))
	case components.ComponentOfLoader:
		return c.loaderHandler.OnStart(ctx, info, document.ConvLoaderCallbackInput(input))
	case components.ComponentOfTransformer:
		return c.transformerHandler.OnStart(ctx, info, document.ConvTransformerCallbackInput(input))
	case components.ComponentOfTool:
		return c.toolHandler.OnStart(ctx, info, tool.ConvCallbackInput(input))
	case compose.ComponentOfToolsNode:
		return c.toolsNodeHandler.OnStart(ctx, info, convToolsNodeCallbackInput(input))
	case compose.ComponentOfGraph,
		compose.ComponentOfChain,
		compose.ComponentOfLambda:
		return c.composeTemplates[info.Component].OnStart(ctx, info, input)
	default:
		return ctx
	}
}

// OnEnd is the callback function for the end event of a component.
// implement the callbacks Handler interface.
// OnEnd 是组件结束事件的回调函数。
// 实现 callbacks Handler 接口。
func (c *handlerTemplate) OnEnd(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
	switch info.Component {
	case components.ComponentOfPrompt:
		return c.promptHandler.OnEnd(ctx, info, prompt.ConvCallbackOutput(output))
	case components.ComponentOfChatModel:
		return c.chatModelHandler.OnEnd(ctx, info, model.ConvCallbackOutput(output))
	case components.ComponentOfEmbedding:
		return c.embeddingHandler.OnEnd(ctx, info, embedding.ConvCallbackOutput(output))
	case components.ComponentOfIndexer:
		return c.indexerHandler.OnEnd(ctx, info, indexer.ConvCallbackOutput(output))
	case components.ComponentOfRetriever:
		return c.retrieverHandler.OnEnd(ctx, info, retriever.ConvCallbackOutput(output))
	case components.ComponentOfLoader:
		return c.loaderHandler.OnEnd(ctx, info, document.ConvLoaderCallbackOutput(output))
	case components.ComponentOfTransformer:
		return c.transformerHandler.OnEnd(ctx, info, document.ConvTransformerCallbackOutput(output))
	case components.ComponentOfTool:
		return c.toolHandler.OnEnd(ctx, info, tool.ConvCallbackOutput(output))
	case compose.ComponentOfToolsNode:
		return c.toolsNodeHandler.OnEnd(ctx, info, convToolsNodeCallbackOutput(output))
	case compose.ComponentOfGraph,
		compose.ComponentOfChain,
		compose.ComponentOfLambda:
		return c.composeTemplates[info.Component].OnEnd(ctx, info, output)
	default:
		return ctx
	}
}

// OnError is the callback function for the error event of a component.
// implement the callbacks Handler interface.
// OnError 是组件错误事件的回调函数。
// 实现 callbacks Handler 接口。
func (c *handlerTemplate) OnError(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
	switch info.Component {
	case components.ComponentOfPrompt:
		return c.promptHandler.OnError(ctx, info, err)
	case components.ComponentOfChatModel:
		return c.chatModelHandler.OnError(ctx, info, err)
	case components.ComponentOfEmbedding:
		return c.embeddingHandler.OnError(ctx, info, err)
	case components.ComponentOfIndexer:
		return c.indexerHandler.OnError(ctx, info, err)
	case components.ComponentOfRetriever:
		return c.retrieverHandler.OnError(ctx, info, err)
	case components.ComponentOfLoader:
		return c.loaderHandler.OnError(ctx, info, err)
	case components.ComponentOfTransformer:
		return c.transformerHandler.OnError(ctx, info, err)
	case components.ComponentOfTool:
		return c.toolHandler.OnError(ctx, info, err)
	case compose.ComponentOfToolsNode:
		return c.toolsNodeHandler.OnError(ctx, info, err)
	case compose.ComponentOfGraph,
		compose.ComponentOfChain,
		compose.ComponentOfLambda:
		return c.composeTemplates[info.Component].OnError(ctx, info, err)
	default:
		return ctx
	}
}

// OnStartWithStreamInput is the callback function for the start event of a component with stream input.
// implement the callbacks Handler interface.
// OnStartWithStreamInput 是带有流式输入的组件开始事件的回调函数。
// 实现 callbacks Handler 接口。
func (c *handlerTemplate) OnStartWithStreamInput(ctx context.Context, info *callbacks.RunInfo, input *schema.StreamReader[callbacks.CallbackInput]) context.Context {
	switch info.Component {
	// currently no components.Component receive stream as input
	case compose.ComponentOfGraph,
		compose.ComponentOfChain,
		compose.ComponentOfLambda:
		return c.composeTemplates[info.Component].OnStartWithStreamInput(ctx, info, input)
	default:
		return ctx
	}
}

// OnEndWithStreamOutput is the callback function for the end event of a component with stream output.
// implement the callbacks Handler interface.
// OnEndWithStreamOutput 是带有流式输出的组件结束事件的回调函数。
// 实现 callbacks Handler 接口。
func (c *handlerTemplate) OnEndWithStreamOutput(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[callbacks.CallbackOutput]) context.Context {
	switch info.Component {
	case components.ComponentOfChatModel:
		return c.chatModelHandler.OnEndWithStreamOutput(ctx, info,
			schema.StreamReaderWithConvert(output, func(item callbacks.CallbackOutput) (*model.CallbackOutput, error) {
				return model.ConvCallbackOutput(item), nil
			}))
	case components.ComponentOfTool:
		return c.toolHandler.OnEndWithStreamOutput(ctx, info,
			schema.StreamReaderWithConvert(output, func(item callbacks.CallbackOutput) (*tool.CallbackOutput, error) {
				return tool.ConvCallbackOutput(item), nil
			}))
	case compose.ComponentOfToolsNode:
		return c.toolsNodeHandler.OnEndWithStreamOutput(ctx, info,
			schema.StreamReaderWithConvert(output, func(item callbacks.CallbackOutput) ([]*schema.Message, error) {
				return convToolsNodeCallbackOutput(item), nil
			}))
	case compose.ComponentOfGraph,
		compose.ComponentOfChain,
		compose.ComponentOfLambda:
		return c.composeTemplates[info.Component].OnEndWithStreamOutput(ctx, info, output)
	default:
		return ctx
	}
}

// Needed checks if the callback handler is needed for the given timing.
// Needed 检查在给定时刻是否需要回调处理器。
func (c *handlerTemplate) Needed(ctx context.Context, info *callbacks.RunInfo, timing callbacks.CallbackTiming) bool {
	if info == nil {
		return false
	}

	switch info.Component {
	case components.ComponentOfChatModel:
		if c.chatModelHandler != nil && c.chatModelHandler.Needed(ctx, info, timing) {
			return true
		}
	case components.ComponentOfEmbedding:
		if c.embeddingHandler != nil && c.embeddingHandler.Needed(ctx, info, timing) {
			return true
		}
	case components.ComponentOfIndexer:
		if c.indexerHandler != nil && c.indexerHandler.Needed(ctx, info, timing) {
			return true
		}
	case components.ComponentOfLoader:
		if c.loaderHandler != nil && c.loaderHandler.Needed(ctx, info, timing) {
			return true
		}
	case components.ComponentOfPrompt:
		if c.promptHandler != nil && c.promptHandler.Needed(ctx, info, timing) {
			return true
		}
	case components.ComponentOfRetriever:
		if c.retrieverHandler != nil && c.retrieverHandler.Needed(ctx, info, timing) {
			return true
		}
	case components.ComponentOfTool:
		if c.toolHandler != nil && c.toolHandler.Needed(ctx, info, timing) {
			return true
		}
	case components.ComponentOfTransformer:
		if c.transformerHandler != nil && c.transformerHandler.Needed(ctx, info, timing) {
			return true
		}
	case compose.ComponentOfToolsNode:
		if c.toolsNodeHandler != nil && c.toolsNodeHandler.Needed(ctx, info, timing) {
			return true
		}
	case compose.ComponentOfGraph,
		compose.ComponentOfChain,
		compose.ComponentOfLambda:
		handler := c.composeTemplates[info.Component]
		if handler != nil {
			checker, ok := handler.(callbacks.TimingChecker)
			if !ok || checker.Needed(ctx, info, timing) {
				return true
			}
		}
	default:
		return false
	}

	return false
}

// LoaderCallbackHandler is the handler for the loader callback.
// LoaderCallbackHandler 是 Loader 回调的处理器。
type LoaderCallbackHandler struct {
	OnStart func(ctx context.Context, runInfo *callbacks.RunInfo, input *document.LoaderCallbackInput) context.Context
	OnEnd   func(ctx context.Context, runInfo *callbacks.RunInfo, output *document.LoaderCallbackOutput) context.Context
	OnError func(ctx context.Context, runInfo *callbacks.RunInfo, err error) context.Context
}

// Needed checks if the callback handler is needed for the given timing.
// Needed 检查在给定时刻是否需要回调处理器。
func (ch *LoaderCallbackHandler) Needed(ctx context.Context, runInfo *callbacks.RunInfo, timing callbacks.CallbackTiming) bool {
	switch timing {
	case callbacks.TimingOnStart:
		return ch.OnStart != nil
	case callbacks.TimingOnEnd:
		return ch.OnEnd != nil
	case callbacks.TimingOnError:
		return ch.OnError != nil
	default:
		return false
	}
}

// TransformerCallbackHandler is the handler for the transformer callback.
// TransformerCallbackHandler 是 Transformer 回调的处理器。
type TransformerCallbackHandler struct {
	OnStart func(ctx context.Context, runInfo *callbacks.RunInfo, input *document.TransformerCallbackInput) context.Context
	OnEnd   func(ctx context.Context, runInfo *callbacks.RunInfo, output *document.TransformerCallbackOutput) context.Context
	OnError func(ctx context.Context, runInfo *callbacks.RunInfo, err error) context.Context
}

// Needed checks if the callback handler is needed for the given timing.
// Needed 检查在给定时刻是否需要回调处理器。
func (ch *TransformerCallbackHandler) Needed(ctx context.Context, runInfo *callbacks.RunInfo, timing callbacks.CallbackTiming) bool {
	switch timing {
	case callbacks.TimingOnStart:
		return ch.OnStart != nil
	case callbacks.TimingOnEnd:
		return ch.OnEnd != nil
	case callbacks.TimingOnError:
		return ch.OnError != nil
	default:
		return false
	}
}

// EmbeddingCallbackHandler is the handler for the embedding callback.
// EmbeddingCallbackHandler 是 Embedding 回调的处理器。
type EmbeddingCallbackHandler struct {
	OnStart func(ctx context.Context, runInfo *callbacks.RunInfo, input *embedding.CallbackInput) context.Context
	OnEnd   func(ctx context.Context, runInfo *callbacks.RunInfo, output *embedding.CallbackOutput) context.Context
	OnError func(ctx context.Context, runInfo *callbacks.RunInfo, err error) context.Context
}

// Needed checks if the callback handler is needed for the given timing.
// Needed 检查在给定时刻是否需要回调处理器。
func (ch *EmbeddingCallbackHandler) Needed(ctx context.Context, runInfo *callbacks.RunInfo, timing callbacks.CallbackTiming) bool {
	switch timing {
	case callbacks.TimingOnStart:
		return ch.OnStart != nil
	case callbacks.TimingOnEnd:
		return ch.OnEnd != nil
	case callbacks.TimingOnError:
		return ch.OnError != nil
	default:
		return false
	}
}

// IndexerCallbackHandler is the handler for the indexer callback.
// IndexerCallbackHandler 是 Indexer 回调的处理器。
type IndexerCallbackHandler struct {
	OnStart func(ctx context.Context, runInfo *callbacks.RunInfo, input *indexer.CallbackInput) context.Context
	OnEnd   func(ctx context.Context, runInfo *callbacks.RunInfo, output *indexer.CallbackOutput) context.Context
	OnError func(ctx context.Context, runInfo *callbacks.RunInfo, err error) context.Context
}

// Needed checks if the callback handler is needed for the given timing.
// Needed 检查在给定时刻是否需要回调处理器。
func (ch *IndexerCallbackHandler) Needed(ctx context.Context, runInfo *callbacks.RunInfo, timing callbacks.CallbackTiming) bool {
	switch timing {
	case callbacks.TimingOnStart:
		return ch.OnStart != nil
	case callbacks.TimingOnEnd:
		return ch.OnEnd != nil
	case callbacks.TimingOnError:
		return ch.OnError != nil
	default:
		return false
	}
}

// ModelCallbackHandler is the handler for the model callback.
// ModelCallbackHandler 是 Model 回调的处理器。
type ModelCallbackHandler struct {
	OnStart               func(ctx context.Context, runInfo *callbacks.RunInfo, input *model.CallbackInput) context.Context
	OnEnd                 func(ctx context.Context, runInfo *callbacks.RunInfo, output *model.CallbackOutput) context.Context
	OnEndWithStreamOutput func(ctx context.Context, runInfo *callbacks.RunInfo, output *schema.StreamReader[*model.CallbackOutput]) context.Context
	OnError               func(ctx context.Context, runInfo *callbacks.RunInfo, err error) context.Context
}

// Needed checks if the callback handler is needed for the given timing.
// Needed 检查在给定时刻是否需要回调处理器。
func (ch *ModelCallbackHandler) Needed(ctx context.Context, runInfo *callbacks.RunInfo, timing callbacks.CallbackTiming) bool {
	switch timing {
	case callbacks.TimingOnStart:
		return ch.OnStart != nil
	case callbacks.TimingOnEnd:
		return ch.OnEnd != nil
	case callbacks.TimingOnError:
		return ch.OnError != nil
	case callbacks.TimingOnEndWithStreamOutput:
		return ch.OnEndWithStreamOutput != nil
	default:
		return false
	}
}

// PromptCallbackHandler is the handler for the callback.
// PromptCallbackHandler 是 Prompt 回调的处理器。
type PromptCallbackHandler struct {
	// OnStart is the callback function for the start of the callback.
	// OnStart 是回调开始时的回调函数。
	OnStart func(ctx context.Context, runInfo *callbacks.RunInfo, input *prompt.CallbackInput) context.Context
	// OnEnd is the callback function for the end of the callback.
	// OnEnd 是回调结束时的回调函数。
	OnEnd func(ctx context.Context, runInfo *callbacks.RunInfo, output *prompt.CallbackOutput) context.Context
	// OnError is the callback function for the error of the callback.
	// OnError 是回调出错时的回调函数。
	OnError func(ctx context.Context, runInfo *callbacks.RunInfo, err error) context.Context
}

// Needed checks if the callback handler is needed for the given timing.
// Needed 检查在给定时刻是否需要回调处理器。
func (ch *PromptCallbackHandler) Needed(ctx context.Context, runInfo *callbacks.RunInfo, timing callbacks.CallbackTiming) bool {
	switch timing {
	case callbacks.TimingOnStart:
		return ch.OnStart != nil
	case callbacks.TimingOnEnd:
		return ch.OnEnd != nil
	case callbacks.TimingOnError:
		return ch.OnError != nil
	default:
		return false
	}
}

// RetrieverCallbackHandler is the handler for the retriever callback.
// RetrieverCallbackHandler 是 Retriever 回调的处理器。
type RetrieverCallbackHandler struct {
	// OnStart is the callback function for the start of the retriever.
	// OnStart 是 Retriever 开始时的回调函数。
	OnStart func(ctx context.Context, runInfo *callbacks.RunInfo, input *retriever.CallbackInput) context.Context
	// OnEnd is the callback function for the end of the retriever.
	// OnEnd 是 Retriever 结束时的回调函数。
	OnEnd func(ctx context.Context, runInfo *callbacks.RunInfo, output *retriever.CallbackOutput) context.Context
	// OnError is the callback function for the error of the retriever.
	// OnError 是 Retriever 出错时的回调函数。
	OnError func(ctx context.Context, runInfo *callbacks.RunInfo, err error) context.Context
}

// Needed checks if the callback handler is needed for the given timing.
// Needed 检查在给定时刻是否需要回调处理器。
func (ch *RetrieverCallbackHandler) Needed(ctx context.Context, runInfo *callbacks.RunInfo, timing callbacks.CallbackTiming) bool {
	switch timing {
	case callbacks.TimingOnStart:
		return ch.OnStart != nil
	case callbacks.TimingOnEnd:
		return ch.OnEnd != nil
	case callbacks.TimingOnError:
		return ch.OnError != nil
	default:
		return false
	}
}

// ToolCallbackHandler is the handler for the tool callback.
// ToolCallbackHandler 是 Tool 回调的处理器。
type ToolCallbackHandler struct {
	OnStart               func(ctx context.Context, info *callbacks.RunInfo, input *tool.CallbackInput) context.Context
	OnEnd                 func(ctx context.Context, info *callbacks.RunInfo, output *tool.CallbackOutput) context.Context
	OnEndWithStreamOutput func(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[*tool.CallbackOutput]) context.Context
	OnError               func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context
}

// Needed checks if the callback handler is needed for the given timing.
// Needed 检查在给定时刻是否需要回调处理器。
func (ch *ToolCallbackHandler) Needed(ctx context.Context, runInfo *callbacks.RunInfo, timing callbacks.CallbackTiming) bool {
	switch timing {
	case callbacks.TimingOnStart:
		return ch.OnStart != nil
	case callbacks.TimingOnEnd:
		return ch.OnEnd != nil
	case callbacks.TimingOnEndWithStreamOutput:
		return ch.OnEndWithStreamOutput != nil
	case callbacks.TimingOnError:
		return ch.OnError != nil
	default:
		return false
	}
}

// ToolsNodeCallbackHandlers defines optional callbacks for the Tools node
// lifecycle events.
// ToolsNodeCallbackHandlers 定义了 Tools 节点生命周期事件的可选回调。
type ToolsNodeCallbackHandlers struct {
	OnStart               func(ctx context.Context, info *callbacks.RunInfo, input *schema.Message) context.Context
	OnEnd                 func(ctx context.Context, info *callbacks.RunInfo, input []*schema.Message) context.Context
	OnEndWithStreamOutput func(ctx context.Context, info *callbacks.RunInfo, output *schema.StreamReader[[]*schema.Message]) context.Context
	OnError               func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context
}

// Needed reports whether a handler is registered for the given timing.
// Needed 报告是否为给定的时间注册了处理器。
func (ch *ToolsNodeCallbackHandlers) Needed(ctx context.Context, runInfo *callbacks.RunInfo, timing callbacks.CallbackTiming) bool {
	switch timing {
	case callbacks.TimingOnStart:
		return ch.OnStart != nil
	case callbacks.TimingOnEnd:
		return ch.OnEnd != nil
	case callbacks.TimingOnEndWithStreamOutput:
		return ch.OnEndWithStreamOutput != nil
	case callbacks.TimingOnError:
		return ch.OnError != nil
	default:
		return false
	}
}

// convToolsNodeCallbackInput converts callback input to ToolsNode input type.
// convToolsNodeCallbackInput 将回调输入转换为 ToolsNode 输入类型。
func convToolsNodeCallbackInput(src callbacks.CallbackInput) *schema.Message {
	switch t := src.(type) {
	case *schema.Message:
		return t
	default:
		return nil
	}
}

// convToolsNodeCallbackOutput converts callback output to ToolsNode output type.
// convToolsNodeCallbackOutput 将回调输出转换为 ToolsNode 输出类型。
func convToolsNodeCallbackOutput(src callbacks.CallbackInput) []*schema.Message {
	switch t := src.(type) {
	case []*schema.Message:
		return t
	default:
		return nil
	}
}
