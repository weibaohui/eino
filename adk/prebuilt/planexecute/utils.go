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
// 该结构体主要用于调试或状态追踪，它通过拦截被包装 Agent 的输出流，
// 在原始输出流结束时，追加一个包含当前 Session 中所有键值对的 AgentEvent。
// 这样可以让调用者（如测试代码或上层 Agent）获取到执行过程中的中间状态。
type outputSessionKVsAgent struct {
	adk.Agent
}

// Run 执行被包装的 Agent，并在执行结束后将 Session 中的键值对作为 AgentEvent 发送。
// 方法逻辑：
// 1. 创建一个新的 AsyncIterator 用于向下游发送事件。
// 2. 启动一个 goroutine 消费被包装 Agent 的输出流。
// 3. 将被包装 Agent 的所有事件原样转发给下游。
// 4. 当被包装 Agent 的流结束时，从 Context 中获取 Session 的所有键值对。
// 5. 构造一个新的 AgentEvent，将 Session 数据放入 Output.CustomizedOutput 中并发送。
// 6. 关闭下游流。
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
// 该函数用于装饰一个 Agent，使其在执行结束时输出 Session 数据。
// 使用场景：
// 在多 Agent 协作（如 PlanExecute）中，Replanner 需要知道 Planner 和 Executor 产生的状态（如 Plan、ExecutedSteps）。
// 通过此包装器，可以将这些状态从 Session 中提取出来，传递给下一个 Agent 或返回给用户。
func agentOutputSessionKVs(ctx context.Context, agent adk.Agent) (adk.Agent, error) {
	return &outputSessionKVsAgent{Agent: agent}, nil
}
