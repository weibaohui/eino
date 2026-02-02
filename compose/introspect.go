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

import (
	"context"
	"reflect"

	"github.com/cloudwego/eino/components"
)

// GraphNodeInfo the info which end users pass in when they are adding nodes to graph.
// GraphNodeInfo 是最终用户在向图中添加节点时传入的信息。
type GraphNodeInfo struct {
	Component             components.Component
	Instance              any
	GraphAddNodeOpts      []GraphAddNodeOpt
	InputType, OutputType reflect.Type // mainly for lambda, whose input and output types cannot be inferred by component type
	// InputType, OutputType 主要用于 lambda，因为其输入和输出类型无法通过组件类型推断
	Name                string
	InputKey, OutputKey string
	GraphInfo           *GraphInfo
	Mappings            []*FieldMapping
}

// GraphInfo the info which end users pass in when they are compiling graph.
// It is used in OnCompile callback, so that users can get the node info and instances.
// Users may need the full details of the graph for observation.
// GraphInfo 是最终用户在编译图时传入的信息。
// 它用于 OnCompile 回调中，以便用户获取节点信息和实例。
// 用户可能需要图的完整详细信息以进行观察。
type GraphInfo struct {
	CompileOptions        []GraphCompileOption
	Nodes                 map[string]GraphNodeInfo
	Edges                 map[string][]string // start node key -> end node keys (control edges)
	// Edges: 起始节点键 -> 结束节点键（控制边）
	DataEdges map[string][]string
	Branches  map[string][]GraphBranch // start node key -> branches
	// Branches: 起始节点键 -> 分支
	InputType, OutputType reflect.Type
	Name                  string
	NewGraphOptions       []NewGraphOption
	GenStateFn            func(context.Context) any
}

// GraphCompileCallback is the callback which will be called when graph compilation finishes.
type GraphCompileCallback interface {
	OnFinish(ctx context.Context, info *GraphInfo)
}
