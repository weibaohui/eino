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

package tool

// Option defines call option for InvokableTool or StreamableTool component, which is part of component interface signature.
// Each tool implementation could define its own options struct and option funcs within its own package,
// then wrap the impl specific option funcs into this type, before passing to InvokableRun or StreamableRun.
//
// Option 定义了 InvokableTool 或 StreamableTool 组件的调用选项，这是组件接口签名的一部分。
// 每个工具实现都可以在其自己的包中定义自己的选项结构体和选项函数，
// 然后在传递给 InvokableRun 或 StreamableRun 之前，将实现特定的选项函数包装到此类型中。
type Option struct {
	implSpecificOptFn any
}

// WrapImplSpecificOptFn wraps the impl specific option functions into Option type.
// T: the type of the impl specific options struct.
// Tool implementations are required to use this function to convert its own option functions into the unified Option type.
// For example, if the tool defines its own options struct:
//
//	type customOptions struct {
//	    conf string
//	}
//
// Then the tool needs to provide an option function as such:
//
//	func WithConf(conf string) Option {
//	    return WrapImplSpecificOptFn(func(o *customOptions) {
//			o.conf = conf
//		}
//	}
//
// .
//
// WrapImplSpecificOptFn 将实现特定的选项函数包装到 Option 类型中。
// T: 实现特定选项结构体的类型。
// 工具实现需要使用此函数将其自己的选项函数转换为统一的 Option 类型。
// 例如，如果工具定义了自己的选项结构体：
//
//	type customOptions struct {
//	    conf string
//	}
//
// 那么工具需要提供如下选项函数：
//
//	func WithConf(conf string) Option {
//	    return WrapImplSpecificOptFn(func(o *customOptions) {
//			o.conf = conf
//		}
//	}
//
// .
func WrapImplSpecificOptFn[T any](optFn func(*T)) Option {
	return Option{
		implSpecificOptFn: optFn,
	}
}

// GetImplSpecificOptions provides tool author the ability to extract their own custom options from the unified Option type.
// T: the type of the impl specific options struct.
// This function should be used within the tool implementation's InvokableRun or StreamableRun functions.
// It is recommended to provide a base T as the first argument, within which the tool author can provide default values for the impl specific options.
// eg.
//
//	type customOptions struct {
//	    conf string
//	}
//	defaultOptions := &customOptions{}
//
//	customOptions := tool.GetImplSpecificOptions(defaultOptions, opts...)
//
// GetImplSpecificOptions 为工具作者提供了从统一的 Option 类型中提取其自定义选项的能力。
// T: 实现特定选项结构体的类型。
// 此函数应在工具实现的 InvokableRun 或 StreamableRun 函数中使用。
// 建议提供一个 base T 作为第一个参数，工具作者可以在其中为实现特定选项提供默认值。
// 例如：
//
//	type customOptions struct {
//	    conf string
//	}
//	defaultOptions := &customOptions{}
//
//	customOptions := tool.GetImplSpecificOptions(defaultOptions, opts...)
func GetImplSpecificOptions[T any](base *T, opts ...Option) *T {
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
