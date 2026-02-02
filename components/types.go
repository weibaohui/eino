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

// Package components defines common interfaces that describe component
// types and callback capabilities used across Eino.
//
// Package components 定义了描述组件类型的通用接口以及 Eino 中使用的回调能力。
package components

// Typer get the type name of one component's implementation
// if Typer exists, the full name of the component instance will be {Typer}{Component} by default
// recommend using Camel Case Naming Style for Typer
//
// Typer 获取组件实现的类型名称。
// 如果 Typer 存在，则组件实例的全名默认为 {Typer}{Component}。
// 建议 Typer 使用驼峰命名法。
type Typer interface {
	GetType() string
}

// GetType returns the type name for a component that implements Typer.
//
// GetType 返回实现 Typer 的组件的类型名称。
func GetType(component any) (string, bool) {
	if typer, ok := component.(Typer); ok {
		return typer.GetType(), true
	}

	return "", false
}

// Checker tells callback aspect status of component's implementation
// When the Checker interface is implemented and returns true, the framework will not start the default aspect.
// Instead, the component will decide the callback execution location and the information to be injected.
//
// Checker 告知组件实现的回调切面状态。
// 当实现了 Checker 接口并返回 true 时，框架将不会启动默认切面。
// 而是由组件决定回调执行位置和注入的信息。
type Checker interface {
	IsCallbacksEnabled() bool
}

// IsCallbacksEnabled reports whether a component implements Checker and enables callbacks.
//
// IsCallbacksEnabled 报告组件是否实现了 Checker 并启用了回调。
func IsCallbacksEnabled(i any) bool {
	if checker, ok := i.(Checker); ok {
		return checker.IsCallbacksEnabled()
	}

	return false
}

// Component names representing the different categories of components.
//
// Component 名称，代表不同类别的组件。
type Component string

const (
	// ComponentOfPrompt identifies chat template components.
	// ComponentOfPrompt 标识 chat template 组件。
	ComponentOfPrompt Component = "ChatTemplate"
	// ComponentOfChatModel identifies chat model components.
	// ComponentOfChatModel 标识 chat model 组件。
	ComponentOfChatModel Component = "ChatModel"
	// ComponentOfEmbedding identifies embedding components.
	// ComponentOfEmbedding 标识 embedding 组件。
	ComponentOfEmbedding Component = "Embedding"
	// ComponentOfIndexer identifies indexer components.
	// ComponentOfIndexer 标识 indexer 组件。
	ComponentOfIndexer Component = "Indexer"
	// ComponentOfRetriever identifies retriever components.
	// ComponentOfRetriever 标识 retriever 组件。
	ComponentOfRetriever Component = "Retriever"
	// ComponentOfLoader identifies loader components.
	// ComponentOfLoader 标识 loader 组件。
	ComponentOfLoader Component = "Loader"
	// ComponentOfTransformer identifies document transformer components.
	// ComponentOfTransformer 标识 document transformer 组件。
	ComponentOfTransformer Component = "DocumentTransformer"
	// ComponentOfTool identifies tool components.
	// ComponentOfTool 标识 tool 组件。
	ComponentOfTool Component = "Tool"
)
