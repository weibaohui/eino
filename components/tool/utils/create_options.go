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

package utils

import (
	"context"
	"reflect"

	"github.com/eino-contrib/jsonschema"
)

// UnmarshalArguments is the function type for unmarshalling the arguments.
//
// UnmarshalArguments 是用于解组参数的函数类型。
type UnmarshalArguments func(ctx context.Context, arguments string) (any, error)

// MarshalOutput is the function type for marshalling the output.
//
// MarshalOutput 是用于编组输出的函数类型。
type MarshalOutput func(ctx context.Context, output any) (string, error)

type toolOptions struct {
	um         UnmarshalArguments
	m          MarshalOutput
	scModifier SchemaModifierFn
}

// Option is the option func for the tool.
//
// Option 是工具的选项函数。
type Option func(o *toolOptions)

// WithUnmarshalArguments wraps the unmarshal arguments option.
// when you want to unmarshal the arguments by yourself, you can use this option.
//
// WithUnmarshalArguments 包装解组参数选项。
// 当你想自己解组参数时，可以使用此选项。
func WithUnmarshalArguments(um UnmarshalArguments) Option {
	return func(o *toolOptions) {
		o.um = um
	}
}

// WithMarshalOutput wraps the marshal output option.
// when you want to marshal the output by yourself, you can use this option.
//
// WithMarshalOutput 包装编组输出选项。
// 当你想自己编组输出时，可以使用此选项。
func WithMarshalOutput(m MarshalOutput) Option {
	return func(o *toolOptions) {
		o.m = m
	}
}

// SchemaModifierFn is the schema modifier function for inferring tool parameter from tagged go struct.
// Within this function, end-user can parse custom go struct tags into corresponding json schema field.
// Parameters:
// 1. jsonTagName: the name defined in the json tag. Specifically, the last 'jsonTagName' visited is fixed to be '_root', which represents the entire go struct. Also, for array field, both the field itself and the element within the array will trigger this function.
// 2. t: the type of current schema, usually the field type of the go struct.
// 3. tag: the struct tag of current schema, usually the field tag of the go struct. Note that the element within an array field will use the same go struct tag as the array field itself.
// 4. schema: the current json schema object to be modified.
//
// SchemaModifierFn 是用于从带标签的 go 结构体推断工具参数的架构修改器函数。
// 在此函数中，最终用户可以将自定义 go 结构体标签解析为相应的 json 架构字段。
// 参数：
// 1. jsonTagName: json 标签中定义的名称。具体来说，最后访问的 'jsonTagName' 固定为 '_root'，代表整个 go 结构体。此外，对于数组字段，字段本身和数组内的元素都会触发此函数。
// 2. t: 当前架构的类型，通常是 go 结构体的字段类型。
// 3. tag: 当前架构的结构体标签，通常是 go 结构体的字段标签。请注意，数组字段内的元素将使用与数组字段本身相同的 go 结构体标签。
// 4. schema: 要修改的当前 json 架构对象。
type SchemaModifierFn func(jsonTagName string, t reflect.Type, tag reflect.StructTag, schema *jsonschema.Schema)

// WithSchemaModifier sets a user-defined schema modifier for inferring tool parameter from tagged go struct.
//
// WithSchemaModifier 设置用户定义的架构修改器，用于从带标签的 go 结构体推断工具参数。
func WithSchemaModifier(modifier SchemaModifierFn) Option {
	return func(o *toolOptions) {
		o.scModifier = modifier
	}
}

func getToolOptions(opt ...Option) *toolOptions {
	opts := &toolOptions{
		um: nil,
		m:  nil,
	}
	for _, o := range opt {
		o(opts)
	}
	return opts
}
