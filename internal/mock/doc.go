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

// Package mock provides mock implementations for testing purposes.
//
// Package mock 提供用于测试目的的模拟实现。
//
// This package aims to provide mock implementations for interfaces in the components package,
// making it easier to use in testing environments. It includes mock implementations for
// various core components such as retrievers, tools, message handlers, and graph runners.
//
// 此包旨在为 components 包中的接口提供模拟实现，使其更容易在测试环境中使用。
// 它包括各种核心组件的模拟实现，如检索器、工具、消息处理程序和图运行器。
//
// Directory Structure:
// 目录结构：
//   - components/: Contains mock implementations for various components
//   - components/: 包含各种组件的模拟实现
//   - retriever/: Provides mock implementation for the Retriever interface
//   - retriever/: 提供 Retriever 接口的模拟实现
//   - retriever_mock.go: Mock implementation for document retrieval
//   - retriever_mock.go: 文档检索的模拟实现
//   - tool/: Mock implementations for tool-related interfaces
//   - tool/: 工具相关接口的模拟实现
//   - message/: Mock implementations for message handling components
//   - message/: 消息处理组件的模拟实现
//   - graph/: Mock implementations for graph execution components
//   - graph/: 图执行组件的模拟实现
//   - stream/: Mock implementations for streaming components
//   - stream/: 流组件的模拟实现
//
// Usage:
// 用法：
// These mock implementations are primarily used in unit tests and integration tests,
// allowing developers to conduct tests without depending on actual external services.
// Each mock component strictly follows the contract of its corresponding interface
// while providing controllable behaviors and results.
//
// 这些模拟实现主要用于单元测试和集成测试，允许开发人员在不依赖实际外部服务的情况下进行测试。
// 每个模拟组件都严格遵循其相应接口的契约，同时提供可控的行为和结果。
//
// Examples:
// 示例：
//
//   - Using mock retriever:
//
//   - 使用模拟检索器：
//     retriever := mock.NewMockRetriever()
//     // Configure retriever behavior
//     // 配置检索器行为
//
//   - Using mock tool:
//
//   - 使用模拟工具：
//     tool := mock.NewMockTool()
//     // Configure tool behavior
//     // 配置工具行为
//
//   - Using mock graph runner:
//
//   - 使用模拟图运行器：
//     runner := mock.NewMockGraphRunner()
//     // Configure runner behavior
//     // 配置运行器行为
package mock
