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

package planexecute

import (
	"context"

	"github.com/cloudwego/eino/adk"
)

// outputSessionKVsAgent 是一个包装器，用于在 Agent 执行结束后输出 Session 中的键值对。
// 它通过拦截 Agent 的输出流，在流结束时追加包含 Session 数据的 AgentEvent。
type outputSessionKVsAgent struct {
	adk.Agent
}

// Run 执行被包装的 Agent，并在执行结束后将 Session 中的键值对作为 AgentEvent 发送。
func (o *outputSessionKVsAgent) Run(ctx context.Context, input *adk.AgentInput,
	options ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {

	iterator, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()

	iterator_ := o.Agent.Run(ctx, input, options...)
	go func() {
		defer generator.Close()
		for {
			event, ok := iterator_.Next()
			if !ok {
				break
			}
			generator.Send(event)
		}

		kvs := adk.GetSessionValues(ctx)

		event := &adk.AgentEvent{
			Output: &adk.AgentOutput{CustomizedOutput: kvs},
		}
		generator.Send(event)
	}()

	return iterator
}

// agentOutputSessionKVs 创建一个 outputSessionKVsAgent 包装器。
// 该包装器确保 Agent 的 Session 数据在执行结束时被捕获并输出，
// 这对于在多 Agent 协作中传递状态（如 Plan、ExecutedSteps）非常重要。
func agentOutputSessionKVs(ctx context.Context, agent adk.Agent) (adk.Agent, error) {
	return &outputSessionKVsAgent{Agent: agent}, nil
}
