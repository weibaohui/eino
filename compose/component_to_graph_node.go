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
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
)

func toComponentNode[I, O, TOption any](
	node any,
	componentType component,
	invoke Invoke[I, O, TOption],
	stream Stream[I, O, TOption],
	collect Collect[I, O, TOption],
	transform Transform[I, O, TOption],
	opts ...GraphAddNodeOpt,
) (*graphNode, *graphAddNodeOpts) {
	meta := parseExecutorInfoFromComponent(componentType, node)
	info, options := getNodeInfo(opts...)
	run := runnableLambda(invoke, stream, collect, transform,
		!meta.isComponentCallbackEnabled,
	)

	gn := toNode(info, run, nil, meta, node, opts...)

	return gn, options
}

// toEmbeddingNode converts an Embedder component to a graph node.
// toEmbeddingNode 将 Embedder 组件转换为图节点。
func toEmbeddingNode(node embedding.Embedder, opts ...GraphAddNodeOpt) (*graphNode, *graphAddNodeOpts) {
	return toComponentNode(
		node,
		components.ComponentOfEmbedding,
		node.EmbedStrings,
		nil,
		nil,
		nil,
		opts...)
}

// toRetrieverNode converts a Retriever component to a graph node.
// toRetrieverNode 将 Retriever 组件转换为图节点。
func toRetrieverNode(node retriever.Retriever, opts ...GraphAddNodeOpt) (*graphNode, *graphAddNodeOpts) {
	return toComponentNode(
		node,
		components.ComponentOfRetriever,
		node.Retrieve,
		nil,
		nil,
		nil,
		opts...)
}

// toLoaderNode converts a Loader component to a graph node.
// toLoaderNode 将 Loader 组件转换为图节点。
func toLoaderNode(node document.Loader, opts ...GraphAddNodeOpt) (*graphNode, *graphAddNodeOpts) {
	return toComponentNode(
		node,
		components.ComponentOfLoader,
		node.Load,
		nil,
		nil,
		nil,
		opts...)
}

// toIndexerNode converts an Indexer component to a graph node.
// toIndexerNode 将 Indexer 组件转换为图节点。
func toIndexerNode(node indexer.Indexer, opts ...GraphAddNodeOpt) (*graphNode, *graphAddNodeOpts) {
	return toComponentNode(
		node,
		components.ComponentOfIndexer,
		node.Store,
		nil,
		nil,
		nil,
		opts...)
}

// toChatModelNode converts a ChatModel component to a graph node.
// toChatModelNode 将 ChatModel 组件转换为图节点。
func toChatModelNode(node model.BaseChatModel, opts ...GraphAddNodeOpt) (*graphNode, *graphAddNodeOpts) {
	return toComponentNode(
		node,
		components.ComponentOfChatModel,
		node.Generate,
		node.Stream,
		nil,
		nil,
		opts...)
}

// toChatTemplateNode converts a ChatTemplate component to a graph node.
// toChatTemplateNode 将 ChatTemplate 组件转换为图节点。
func toChatTemplateNode(node prompt.ChatTemplate, opts ...GraphAddNodeOpt) (*graphNode, *graphAddNodeOpts) {
	return toComponentNode(
		node,
		components.ComponentOfPrompt,
		node.Format,
		nil,
		nil,
		nil,
		opts...)
}

// toDocumentTransformerNode converts a DocumentTransformer component to a graph node.
// toDocumentTransformerNode 将 DocumentTransformer 组件转换为图节点。
func toDocumentTransformerNode(node document.Transformer, opts ...GraphAddNodeOpt) (*graphNode, *graphAddNodeOpts) {
	return toComponentNode(
		node,
		components.ComponentOfTransformer,
		node.Transform,
		nil,
		nil,
		nil,
		opts...)
}

// toToolsNode converts a ToolsNode to a graph node.
// toToolsNode 将 ToolsNode 转换为图节点。
func toToolsNode(node *ToolsNode, opts ...GraphAddNodeOpt) (*graphNode, *graphAddNodeOpts) {
	return toComponentNode(
		node,
		ComponentOfToolsNode,
		node.Invoke,
		node.Stream,
		nil,
		nil,
		opts...)
}

// toLambdaNode converts a Lambda to a graph node.
// toLambdaNode 将 Lambda 转换为图节点。
func toLambdaNode(node *Lambda, opts ...GraphAddNodeOpt) (*graphNode, *graphAddNodeOpts) {
	info, options := getNodeInfo(opts...)

	gn := toNode(info, node.executor, nil, node.executor.meta, node, opts...)

	return gn, options
}

// toAnyGraphNode converts an AnyGraph to a graph node.
// toAnyGraphNode 将 AnyGraph 转换为图节点。
func toAnyGraphNode(node AnyGraph, opts ...GraphAddNodeOpt) (*graphNode, *graphAddNodeOpts) {
	meta := parseExecutorInfoFromComponent(node.component(), node)
	info, options := getNodeInfo(opts...)

	gn := toNode(info, nil, node, meta, node, opts...)

	return gn, options
}

// toPassthroughNode creates a Passthrough node.
// toPassthroughNode 创建一个 Passthrough 节点。
func toPassthroughNode(opts ...GraphAddNodeOpt) (*graphNode, *graphAddNodeOpts) {
	node := composablePassthrough()
	info, options := getNodeInfo(opts...)
	gn := toNode(info, node, nil, node.meta, node, opts...)
	return gn, options
}

// toNode creates a new graphNode with the provided information.
// toNode 使用提供的信息创建一个新的 graphNode。
func toNode(nodeInfo *nodeInfo, executor *composableRunnable, graph AnyGraph,
	meta *executorMeta, instance any, opts ...GraphAddNodeOpt) *graphNode {

	if meta == nil {
		meta = &executorMeta{}
	}

	gn := &graphNode{
		nodeInfo: nodeInfo,

		cr:           executor,
		g:            graph,
		executorMeta: meta,

		instance: instance,
		opts:     opts,
	}

	return gn
}
