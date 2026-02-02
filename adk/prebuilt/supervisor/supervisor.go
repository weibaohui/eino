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

// Config is the configuration for the Supervisor agent.
// Config 是 Supervisor Agent 的配置。
type Config struct {
	// Supervisor is the agent that coordinates the sub-agents.
	// It decides which sub-agent to delegate the task to or whether to respond to the user.
	// Supervisor 是协调 Sub-Agent 的 Agent。
	// 它决定将任务委派给哪个 Sub-Agent，或者是否响应用户。
	Supervisor adk.Agent

	// SubAgents is the list of agents that the supervisor can delegate tasks to.
	// SubAgents 是 Supervisor 可以委派任务的 Agent 列表。
	SubAgents []adk.Agent
}

// New creates a new Supervisor based multi-agent system.
// In the Supervisor pattern, a supervisor agent coordinates multiple sub-agents.
// The supervisor can delegate tasks to sub-agents and receive their responses,
// but sub-agents can only communicate with the supervisor (not with each other directly).
// New 创建一个新的基于 Supervisor 模式的多 Agent 系统。
// 在 Supervisor 模式中，一个 Supervisor Agent 协调多个 Sub-Agent。
// Supervisor 可以将任务委派给 Sub-Agent 并接收它们的响应，
// 但 Sub-Agent 只能与 Supervisor 通信（不能直接相互通信）。
func New(ctx context.Context, conf *Config) (adk.ResumableAgent, error) {
	subAgents := make([]adk.Agent, 0, len(conf.SubAgents))
	supervisorName := conf.Supervisor.Name(ctx)
	for _, subAgent := range conf.SubAgents {
		subAgents = append(subAgents, adk.AgentWithDeterministicTransferTo(ctx, &adk.DeterministicTransferConfig{
			Agent:        subAgent,
			ToAgentNames: []string{supervisorName},
		}))
	}
	return adk.SetSubAgents(ctx, conf.Supervisor, subAgents)
}
