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

// options 是 Agent 运行时的内部选项集合。
type options struct {
	// sharedParentSession 指示是否与父级共享会话。
	sharedParentSession bool
	// sessionValues 存储会话级别的值。
	sessionValues map[string]any
	// checkPointID 指定用于恢复的检查点 ID。
	checkPointID *string
	// skipTransferMessages 指示是否跳过转发 Transfer 消息。
	skipTransferMessages bool
}

// AgentRunOption 是 adk Agent 的调用选项。
// 它封装了特定实现的选项函数，并支持指定生效的 Agent 名称。
type AgentRunOption struct {
	// implSpecificOptFn 特定实现的选项函数。
	implSpecificOptFn any

	// agentNames 指定可以看到此 AgentRunOption 的 Agent 名称列表。
	// 如果为空，则所有 Agent 都可以看到此选项。
	agentNames []string
}

// DesignateAgent 指定此选项仅对特定的 Agent 生效。
// 为什么要做这个：在多 Agent 系统中，某些配置可能只适用于特定的 Agent。
// 如何使用：opt.DesignateAgent("agent1", "agent2")
func (o AgentRunOption) DesignateAgent(name ...string) AgentRunOption {
	o.agentNames = append(o.agentNames, name...)
	return o
}

// getCommonOptions 从选项列表中提取通用选项（options 结构体）。
// 为什么要做这个：将通用的配置逻辑集中处理。
func getCommonOptions(base *options, opts ...AgentRunOption) *options {
	if base == nil {
		base = &options{}
	}

	return GetImplSpecificOptions[options](base, opts...)
}

// WithSessionValues 为 Agent 运行设置会话范围的值。
// 为什么要做这个：允许在一次运行会话中传递上下文信息，供 Agent 内部使用（如 Prompt 模板填充）。
// 如何使用：传入一个 map，其中包含键值对。
func WithSessionValues(v map[string]any) AgentRunOption {
	return WrapImplSpecificOptFn(func(o *options) {
		o.sessionValues = v
	})
}

// WithSkipTransferMessages 禁用在执行期间转发 Transfer 消息。
// 为什么要做这个：有时我们不希望 Transfer 消息被自动处理或显示。
// 如何使用：作为 Agent 运行选项传入。
func WithSkipTransferMessages() AgentRunOption {
	return WrapImplSpecificOptFn(func(t *options) {
		t.skipTransferMessages = true
	})
}

// withSharedParentSession 设置共享父会话选项。
// 这是一个内部选项，用于嵌套 Agent 场景。
func withSharedParentSession() AgentRunOption {
	return WrapImplSpecificOptFn(func(o *options) {
		o.sharedParentSession = true
	})
}

// WrapImplSpecificOptFn 用于包装特定实现的选项函数。
// 为什么要做这个：为了让通用的 AgentRunOption 能够携带特定实现的配置逻辑。
func WrapImplSpecificOptFn[T any](optFn func(*T)) AgentRunOption {
	return AgentRunOption{
		implSpecificOptFn: optFn,
	}
}

// GetImplSpecificOptions 从 AgentRunOption 列表中提取特定实现的选项。
// 可选地提供一个带有默认值的 base 选项对象。
// 为什么要做这个：允许不同的 Agent 实现定义和获取自己的配置选项。
// 如何使用：
//
//	myOption := &MyOption{
//		Field1: "default_value",
//	}
//
//	myOption := model.GetImplSpecificOptions(myOption, opts...)
func GetImplSpecificOptions[T any](base *T, opts ...AgentRunOption) *T {
	if base == nil {
		base = new(T)
	}

	for i := range opts {
		opt := opts[i]
		if opt.implSpecificOptFn != nil {
			optFn, ok := opt.implSpecificOptFn.(func(*T))
			if ok {
				optFn(base)
			}
		}
	}

	return base
}

// filterOptions 根据 Agent 名称过滤选项。
// 为什么要做这个：确保 Agent 只接收到针对它的配置选项或全局选项。
func filterOptions(agentName string, opts []AgentRunOption) []AgentRunOption {
	if len(opts) == 0 {
		return nil
	}
	var filteredOpts []AgentRunOption
	for i := range opts {
		opt := opts[i]
		if len(opt.agentNames) == 0 {
			filteredOpts = append(filteredOpts, opt)
			continue
		}
		for j := range opt.agentNames {
			if opt.agentNames[j] == agentName {
				filteredOpts = append(filteredOpts, opt)
				break
			}
		}
	}
	return filteredOpts
}

 
