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

import "fmt"

// pregelChannelBuilder creates a new pregelChannel.
// pregelChannelBuilder 创建一个新的 pregelChannel。
func pregelChannelBuilder(_ []string, _ []string, _ func() any, _ func() streamReader) channel {
	return &pregelChannel{Values: make(map[string]any)}
}

// pregelChannel implements the channel interface for Pregel graph execution.
// pregelChannel 实现了 Pregel 图执行的 channel 接口。
type pregelChannel struct {
	Values map[string]any

	mergeConfig FanInMergeConfig
}

// setMergeConfig sets the merge configuration for the channel.
// setMergeConfig 设置通道的合并配置。
func (ch *pregelChannel) setMergeConfig(cfg FanInMergeConfig) {
	ch.mergeConfig.StreamMergeWithSourceEOF = cfg.StreamMergeWithSourceEOF
}

// load loads values from another channel.
// load 从另一个通道加载值。
func (ch *pregelChannel) load(c channel) error {
	dc, ok := c.(*pregelChannel)
	if !ok {
		return fmt.Errorf("load pregel channel fail, got %T, want *pregelChannel", c)
	}
	ch.Values = dc.Values
	return nil
}

// convertValues applies a function to the channel's values.
// convertValues 对通道的值应用一个函数。
func (ch *pregelChannel) convertValues(fn func(map[string]any) error) error {
	return fn(ch.Values)
}

// reportValues adds values to the channel.
// reportValues 向通道添加值。
func (ch *pregelChannel) reportValues(ins map[string]any) error {
	for k, v := range ins {
		ch.Values[k] = v
	}
	return nil
}

// get retrieves a value from the channel.
// get 从通道中检索值。
func (ch *pregelChannel) get(isStream bool, name string, edgeHandler *edgeHandlerManager) (
	any, bool, error) {
	if len(ch.Values) == 0 {
		return nil, false, nil
	}
	defer func() { ch.Values = map[string]any{} }()
	values := make([]any, len(ch.Values))
	names := make([]string, len(ch.Values))
	i := 0
	for k, v := range ch.Values {
		resolvedV, err := edgeHandler.handle(k, name, v, isStream)
		if err != nil {
			return nil, false, err
		}
		values[i] = resolvedV
		names[i] = k
		i++
	}

	if len(values) == 1 {
		return values[0], true, nil
	}

	// merge
	mergeOpts := &mergeOptions{
		streamMergeWithSourceEOF: ch.mergeConfig.StreamMergeWithSourceEOF,
		names:                    names,
	}
	v, err := mergeValues(values, mergeOpts)
	if err != nil {
		return nil, false, err
	}
	return v, true, nil
}

// reportSkip reports that the node execution should be skipped.
// reportSkip 报告应跳过节点执行。
func (ch *pregelChannel) reportSkip(_ []string) bool {
	return false
}

// reportDependencies reports the dependencies of the node.
// reportDependencies 报告节点的依赖关系。
func (ch *pregelChannel) reportDependencies(_ []string) {
	return
}
