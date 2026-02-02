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

package schema

import (
	"sort"

	"github.com/eino-contrib/jsonschema"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

// DataType is the type of the parameter.
// It must be one of the following values: "object", "number", "integer", "string", "array", "null", "boolean", which is the same as the type of the parameter in JSONSchema.
// DataType 是参数的类型。
// 它必须是以下值之一："object", "number", "integer", "string", "array", "null", "boolean"，这与 JSONSchema 中的参数类型相同。
type DataType string

// Supported JSONSchema data types for tool parameters.
// 工具参数支持的 JSONSchema 数据类型。
const (
	Object  DataType = "object"
	Number  DataType = "number"
	Integer DataType = "integer"
	String  DataType = "string"
	Array   DataType = "array"
	Null    DataType = "null"
	Boolean DataType = "boolean"
)

// ToolChoice controls how the model calls tools (if any).
// ToolChoice 控制模型如何调用工具（如果有）。
type ToolChoice string

const (
	// ToolChoiceForbidden indicates that the model should not call any tools.
	// Corresponds to "none" in OpenAI Chat Completion.
	// ToolChoiceForbidden 表示模型不应调用任何工具。
	// 对应于 OpenAI Chat Completion 中的 "none"。
	ToolChoiceForbidden ToolChoice = "forbidden"

	// ToolChoiceAllowed indicates that the model can choose to generate a message or call one or more tools.
	// Corresponds to "auto" in OpenAI Chat Completion.
	// ToolChoiceAllowed 表示模型可以选择生成消息或调用一个或多个工具。
	// 对应于 OpenAI Chat Completion 中的 "auto"。
	ToolChoiceAllowed ToolChoice = "allowed"

	// ToolChoiceForced indicates that the model must call one or more tools.
	// Corresponds to "required" in OpenAI Chat Completion.
	// ToolChoiceForced 表示模型必须调用一个或多个工具。
	// 对应于 OpenAI Chat Completion 中的 "required"。
	ToolChoiceForced ToolChoice = "forced"
)

// ToolInfo is the information of a tool.
// ToolInfo 是工具的信息。
type ToolInfo struct {
	// The unique name of the tool that clearly communicates its purpose.
	// 工具的唯一名称，清楚地表达其用途。
	Name string
	// Used to tell the model how/when/why to use the tool.
	// You can provide few-shot examples as a part of the description.
	// 用于告诉模型如何/何时/为什么使用该工具。
	// 您可以提供少样本示例作为描述的一部分。
	Desc string
	// Extra is the extra information for the tool.
	// Extra 是工具的额外信息。
	Extra map[string]any

	// The parameters the functions accepts (different models may require different parameter types).
	// can be described in two ways:
	//  - use params: schema.NewParamsOneOfByParams(params)
	//  - use jsonschema: schema.NewParamsOneOfByJSONSchema(jsonschema)
	// If is nil, signals that the tool does not need any input parameter
	// 函数接受的参数（不同的模型可能需要不同的参数类型）。
	// 可以用两种方式描述：
	//  - 使用 params: schema.NewParamsOneOfByParams(params)
	//  - 使用 jsonschema: schema.NewParamsOneOfByJSONSchema(jsonschema)
	// 如果为 nil，表示该工具不需要任何输入参数
	*ParamsOneOf
}

// ParameterInfo is the information of a parameter.
// It is used to describe the parameters of a tool.
// ParameterInfo 是参数的信息。
// 它用于描述工具的参数。
type ParameterInfo struct {
	// The type of the parameter.
	// 参数的类型。
	Type DataType
	// The element type of the parameter, only for array.
	// 参数的元素类型，仅用于数组。
	ElemInfo *ParameterInfo
	// The sub parameters of the parameter, only for object.
	// 参数的子参数，仅用于对象。
	SubParams map[string]*ParameterInfo
	// The description of the parameter.
	// 参数的描述。
	Desc string
	// The enum values of the parameter, only for string.
	// 参数的枚举值，仅用于字符串。
	Enum []string
	// Whether the parameter is required.
	// 参数是否必须。
	Required bool
}

// ParamsOneOf is a union of the different methods user can choose which describe a tool's request parameters.
// User must specify one and ONLY one method to describe the parameters.
//  1. use NewParamsOneOfByParams(): an intuitive way to describe the parameters that covers most of the use-cases.
//  2. use NewParamsOneOfByJSONSchema(): a formal way to describe the parameters that strictly adheres to JSONSchema specification.
//     See https://json-schema.org/draft/2020-12.
//
// ParamsOneOf 是用户可以选择用来描述工具请求参数的不同方法的联合。
// 用户必须指定一种且仅一种方法来描述参数。
//  1. 使用 NewParamsOneOfByParams()：一种直观的描述参数的方法，涵盖了大多数用例。
//  2. 使用 NewParamsOneOfByJSONSchema()：一种严格遵守 JSONSchema 规范的描述参数的正式方法。
//     参见 https://json-schema.org/draft/2020-12。
type ParamsOneOf struct {
	// use NewParamsOneOfByParams to set this field
	params map[string]*ParameterInfo

	jsonschema *jsonschema.Schema
}

// NewParamsOneOfByParams creates a ParamsOneOf with map[string]*ParameterInfo.
// NewParamsOneOfByParams 使用 map[string]*ParameterInfo 创建 ParamsOneOf。
func NewParamsOneOfByParams(params map[string]*ParameterInfo) *ParamsOneOf {
	return &ParamsOneOf{
		params: params,
	}
}

// NewParamsOneOfByJSONSchema creates a ParamsOneOf with *jsonschema.Schema.
// NewParamsOneOfByJSONSchema 使用 *jsonschema.Schema 创建 ParamsOneOf。
func NewParamsOneOfByJSONSchema(s *jsonschema.Schema) *ParamsOneOf {
	return &ParamsOneOf{
		jsonschema: s,
	}
}

// ToJSONSchema parses ParamsOneOf, converts the parameter description that user actually provides, into the format ready to be passed to Model.
// ToJSONSchema 解析 ParamsOneOf，将用户实际提供的参数描述转换为准备传递给模型的格式。
func (p *ParamsOneOf) ToJSONSchema() (*jsonschema.Schema, error) {
	if p == nil {
		return nil, nil
	}

	if p.params != nil {
		sc := &jsonschema.Schema{
			Properties: orderedmap.New[string, *jsonschema.Schema](),
			Type:       string(Object),
			Required:   make([]string, 0, len(p.params)),
		}

		keys := make([]string, 0, len(p.params))
		for k := range p.params {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			v := p.params[k]
			sc.Properties.Set(k, paramInfoToJSONSchema(v))
			if v.Required {
				sc.Required = append(sc.Required, k)
			}
		}

		return sc, nil
	}

	return p.jsonschema, nil
}

func paramInfoToJSONSchema(paramInfo *ParameterInfo) *jsonschema.Schema {
	js := &jsonschema.Schema{
		Type:        string(paramInfo.Type),
		Description: paramInfo.Desc,
	}

	if len(paramInfo.Enum) > 0 {
		js.Enum = make([]any, len(paramInfo.Enum))
		for i, enum := range paramInfo.Enum {
			js.Enum[i] = enum
		}
	}

	if paramInfo.ElemInfo != nil {
		js.Items = paramInfoToJSONSchema(paramInfo.ElemInfo)
	}

	if len(paramInfo.SubParams) > 0 {
		required := make([]string, 0, len(paramInfo.SubParams))
		js.Properties = orderedmap.New[string, *jsonschema.Schema]()
		keys := make([]string, 0, len(paramInfo.SubParams))
		for k := range paramInfo.SubParams {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			v := paramInfo.SubParams[k]
			item := paramInfoToJSONSchema(v)
			js.Properties.Set(k, item)
			if v.Required {
				required = append(required, k)
			}
		}

		js.Required = required
	}

	return js
}
