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
	"context"
	"fmt"
	"strings"
)

// TransferToAgentInstruction 是用于指导模型如何进行 Agent 转移的系统指令模板。
// 它列出了可用的 Agent，并说明了决策规则。
// 为什么要做这个：在多 Agent 协作场景中，当一个 Agent 无法处理当前问题时，需要将其转移给更合适的 Agent。
// 如何使用：通过 fmt.Sprintf 将可用 Agent 列表和转移工具名称填充到模板中。
const (
	TransferToAgentInstruction = `Available other agents: %s

Decision rule:
- If you're best suited for the question according to your description: ANSWER
- If another agent is better according its description: CALL '%s' function with their agent name

When transferring: OUTPUT ONLY THE FUNCTION CALL`
)

// genTransferToAgentInstruction 生成转移 Agent 的系统指令。
// 为什么要做这个：动态生成包含所有可选 Agent 信息（名称和描述）的指令，告诉 LLM 当前有哪些 Agent 可用，以及如何调用工具进行转移。
// 如何使用：传入上下文和可用 Agent 列表，返回格式化后的完整指令字符串。
func genTransferToAgentInstruction(ctx context.Context, agents []Agent) string {
	var sb strings.Builder
	for _, agent := range agents {
		sb.WriteString(fmt.Sprintf("\n- Agent name: %s\n  Agent description: %s",
			agent.Name(ctx), agent.Description(ctx)))
	}

	return fmt.Sprintf(TransferToAgentInstruction, sb.String(), TransferToAgentToolName)
}
