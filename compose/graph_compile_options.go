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

type graphCompileOptions struct {
	maxRunSteps     int
	graphName       string
	nodeTriggerMode NodeTriggerMode // default to AnyPredecessor (pregel)

	callbacks []GraphCompileCallback

	origOpts []GraphCompileOption

	checkPointStore      CheckPointStore
	serializer           Serializer
	interruptBeforeNodes []string
	interruptAfterNodes  []string

	eagerDisabled bool

	mergeConfigs map[string]FanInMergeConfig
}

func newGraphCompileOptions(opts ...GraphCompileOption) *graphCompileOptions {
	option := &graphCompileOptions{}

	for _, o := range opts {
		o(option)
	}

	option.origOpts = opts

	return option
}

// GraphCompileOption options for compiling AnyGraph.
// GraphCompileOption 用于编译 AnyGraph 的选项。
type GraphCompileOption func(*graphCompileOptions)

// WithMaxRunSteps sets the maximum number of steps that a graph can run.
// This is useful to prevent infinite loops in graphs with cycles.
// If the number of steps exceeds maxSteps, the graph execution will be terminated with an error.
// WithMaxRunSteps 设置图运行的最大步数。
// 这对于防止循环图中的无限循环很有用。
// 如果步数超过 maxSteps，图执行将以错误终止。
func WithMaxRunSteps(maxSteps int) GraphCompileOption {
	return func(o *graphCompileOptions) {
		o.maxRunSteps = maxSteps
	}
}

// WithGraphName sets a name for the graph.
// The name is used for debugging and logging purposes.
// If not set, a default name will be used.
// WithGraphName 为图设置名称。
// 名称用于调试和日志记录目的。
// 如果未设置，将使用默认名称。
func WithGraphName(graphName string) GraphCompileOption {
	return func(o *graphCompileOptions) {
		o.graphName = graphName
	}
}

// WithEagerExecution enables the eager execution mode for the graph.
// In eager mode, nodes will be executed immediately once they are ready to run,
// without waiting for the completion of a super step, ref: https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/orchestration_design_principles/#runtime-engine
// Note: Eager mode is not allowed when the graph's trigger mode is set to AnyPredecessor.
// Workflow uses eager mode by default.
// Deprecated: Eager execution is automatically enabled by default when a node's trigger mode is set to AllPredecessor.
// If you were using this option previously, it can be safely removed without changing behavior.
// WithEagerExecution 启用图的急切执行模式。
// 在急切模式下，节点一旦准备好运行就会立即执行，
// 而无需等待超级步的完成，参考：https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/orchestration_design_principles/#runtime-engine
// 注意：当图的触发模式设置为 AnyPredecessor 时，不允许使用急切模式。
// Workflow 默认使用急切模式。
// 已弃用：当节点的触发模式设置为 AllPredecessor 时，急切执行默认自动启用。
// 如果您以前使用此选项，可以安全地将其删除，而不会改变行为。
func WithEagerExecution() GraphCompileOption {
	return func(o *graphCompileOptions) {
		return
	}
}

// WithEagerExecutionDisabled disables the eager execution mode for the graph.
// By default, eager execution is enabled for Workflow and Graph with the AllPredecessor trigger mode.
// After using this option, nodes will wait for the completion of a super step instead of execute immediately once they are ready to run.
// ref: https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/orchestration_design_principles/#runtime-engine
// WithEagerExecutionDisabled 禁用图的急切执行模式。
// 默认情况下，Workflow 和具有 AllPredecessor 触发模式的 Graph 启用了急切执行。
// 使用此选项后，节点将等待超级步的完成，而不是在准备好运行时立即执行。
// 参考：https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/orchestration_design_principles/#runtime-engine
func WithEagerExecutionDisabled() GraphCompileOption {
	return func(o *graphCompileOptions) {
		o.eagerDisabled = true
	}
}

// WithNodeTriggerMode sets the trigger mode for nodes in the graph.
// The trigger mode determines when a node is triggered during graph execution, ref: https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/orchestration_design_principles/#runtime-engine
// AnyPredecessor by default.
// WithNodeTriggerMode 设置图中节点的触发模式。
// 触发模式决定了节点在图执行期间何时被触发，参考：https://www.cloudwego.io/docs/eino/core_modules/chain_and_graph_orchestration/orchestration_design_principles/#runtime-engine
// 默认为 AnyPredecessor。
func WithNodeTriggerMode(triggerMode NodeTriggerMode) GraphCompileOption {
	return func(o *graphCompileOptions) {
		o.nodeTriggerMode = triggerMode
	}
}

// WithGraphCompileCallbacks sets callbacks for graph compilation.
// WithGraphCompileCallbacks 设置图编译的回调函数。
func WithGraphCompileCallbacks(cbs ...GraphCompileCallback) GraphCompileOption {
	return func(o *graphCompileOptions) {
		o.callbacks = append(o.callbacks, cbs...)
	}
}

// FanInMergeConfig defines the configuration for fan-in merge operations.
// It allows specifying how multiple inputs are merged into a single input.
// StreamMergeWithSourceEOF indicates whether to emit a SourceEOF error for each stream
// when it ends, before the final merged output is produced. This is useful for
// tracking the completion of individual input streams in a named stream merge.
// FanInMergeConfig 定义扇入合并操作的配置。
// 它允许指定如何将多个输入合并为单个输入。
// StreamMergeWithSourceEOF 指示是否在每个流结束时（在生成最终合并输出之前）发出 SourceEOF 错误。
// 这对于跟踪命名流合并中各个输入流的完成情况很有用。
type FanInMergeConfig struct {
	StreamMergeWithSourceEOF bool //indicates whether to emit a SourceEOF error for each stream
}

// WithFanInMergeConfig sets the fan-in merge configurations
// for the graph nodes that receive inputs from multiple sources.
// WithFanInMergeConfig 为接收来自多个源的输入的图节点设置扇入合并配置。
func WithFanInMergeConfig(confs map[string]FanInMergeConfig) GraphCompileOption {
	return func(o *graphCompileOptions) {
		o.mergeConfigs = confs
	}
}

// InitGraphCompileCallbacks set global graph compile callbacks,
// which ONLY will be added to top level graph compile options
// InitGraphCompileCallbacks 设置全局图编译回调函数，
// 这些回调函数将仅添加到顶层图编译选项中
func InitGraphCompileCallbacks(cbs []GraphCompileCallback) {
	globalGraphCompileCallbacks = cbs
}

var globalGraphCompileCallbacks []GraphCompileCallback
