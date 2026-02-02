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

// Package supervisor implements the supervisor pattern for multi-agent systems,
// where a central agent coordinates a set of sub-agents.
package supervisor

import (
	"context"

	"github.com/cloudwego/eino/adk"
)

// Config 定义了 Supervisor 模式 Agent 的配置。
// 包含一个作为管理者的 Supervisor Agent 和一组作为工人的 SubAgents。
type Config struct {
	// Supervisor 是负责协调和分派任务的 Agent。
	// 它接收用户输入或子 Agent 的结果，并决定下一步是调用某个子 Agent 还是结束任务。
	Supervisor adk.Agent

	// SubAgents 是可供 Supervisor 调度的子 Agent 列表。
	// 每个子 Agent 专注于特定的领域或任务。
	SubAgents []adk.Agent
}

// New 创建一个新的基于 Supervisor 模式的多 Agent 系统。
//
// 在 Supervisor 模式中，一个指定的 Supervisor Agent 协调多个 Sub-Agent。
// Supervisor 可以将任务委派给 Sub-Agent 并接收它们的响应，
// 而 Sub-Agent 只能与 Supervisor 通信（不能直接相互通信）。
// 这种层级结构使得系统能够通过协调的 Agent 交互来解决复杂问题。
//
// 实现细节：
// 为每个 Sub-Agent 配置了确定性的转移路径（DeterministicTransfer），
// 确保它们执行完毕后，控制权总是交还给 Supervisor。
func New(ctx context.Context, conf *Config) (adk.ResumableAgent, error) {
	subAgents := make([]adk.Agent, 0, len(conf.SubAgents))
	supervisorName := conf.Supervisor.Name(ctx)
	for _, subAgent := range conf.SubAgents {
		// 配置子 Agent：执行完成后自动转移回 Supervisor
		subAgents = append(subAgents, adk.AgentWithDeterministicTransferTo(ctx, &adk.DeterministicTransferConfig{
			Agent:        subAgent,
			ToAgentNames: []string{supervisorName},
		}))
	}

	// 使用 SetSubAgents 构建包含 Supervisor 和所有配置好的 Sub-Agents 的系统
	return adk.SetSubAgents(ctx, conf.Supervisor, subAgents)
}
