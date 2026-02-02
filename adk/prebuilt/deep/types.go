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

package deep

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
)

const (
	// generalAgentName 是通用 Agent 的类型标识。
	generalAgentName = "general-purpose"
	// taskToolName 是 task 工具的名称标识。
	taskToolName = "task"
)

const (
	// SessionKeyTodos 是在 Session 中存储 TODO 列表的键名。
	SessionKeyTodos = "deep_agent_session_key_todos"
)

// assertAgentTool 将一个通用工具转换为可调用的 InvokableTool。
// 这里的工具实际上是封装了 Agent 的工具。
func assertAgentTool(t tool.BaseTool) (tool.InvokableTool, error) {
	it, ok := t.(tool.InvokableTool)
	if !ok {
		info, _ := t.Info(context.Background())
		return nil, fmt.Errorf("agent tool %s is not invokable", info.Name)
	}
	return it, nil
}
