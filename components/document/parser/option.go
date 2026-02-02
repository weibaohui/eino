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

package parser

// Options configures the document parser with source URI and extra metadata.
//
// Options 配置文档解析器的源 URI 和额外元数据。
type Options struct {
	// uri of source.
	// uri of source.
	URI string

	// extra metadata will merge to each document.
	// extra metadata 将合并到每个文档中。
	ExtraMeta map[string]any
}

// Option defines call option for Parser component, which is part of the component interface signature.
// Each Parser implementation could define its own options struct and option funcs within its own package,
// then wrap the impl specific option funcs into this type, before passing to Transform.
//
// Option 定义 Parser 组件的调用选项，它是组件接口签名的一部分。
// 每个 Parser 实现可以在其自己的包中定义自己的 options 结构体和 option 函数，
// 然后在传递给 Transform 之前，将特定于实现的 option 函数包装到此类型中。
type Option struct {
	apply func(opts *Options)

	implSpecificOptFn any
}

// WithURI specifies the source URI of the document.
// It will be used as to select parser in ExtParser.
//
// WithURI 指定文档的源 URI。
// 它将在 ExtParser 中用于选择解析器。
func WithURI(uri string) Option {
	return Option{
		apply: func(opts *Options) {
			opts.URI = uri
		},
	}
}

// WithExtraMeta attaches extra metadata to the parsed document.
//
// WithExtraMeta 将额外元数据附加到解析后的文档。
func WithExtraMeta(meta map[string]any) Option {
	return Option{
		apply: func(opts *Options) {
			opts.ExtraMeta = meta
		},
	}
}

// GetCommonOptions extract parser Options from Option list, optionally providing a base Options with default values.
//
// GetCommonOptions 从 Option 列表中提取 parser Options，可选择提供带有默认值的 base Options。
func GetCommonOptions(base *Options, opts ...Option) *Options {
	if base == nil {
		base = &Options{}
	}

	for i := range opts {
		opt := opts[i]
		if opt.apply != nil {
			opt.apply(base)
		}
	}

	return base
}

// WrapImplSpecificOptFn wraps the impl specific option functions into Option type.
// T: the type of the impl specific options struct.
// Parser implementations are required to use this function to convert its own option functions into the unified Option type.
// For example, if the Parser impl defines its own options struct:
//
//	type customOptions struct {
//	    conf string
//	}
//
// Then the impl needs to provide an option function as such:
//
//	func WithConf(conf string) Option {
//	    return WrapImplSpecificOptFn(func(o *customOptions) {
//			o.conf = conf
//		}
//	}
//
// WrapImplSpecificOptFn 将特定于实现的 option 函数包装到 Option 类型中。
// T：特定于实现的 options 结构体的类型。
// Parser 实现需要使用此函数将其自己的 option 函数转换为统一的 Option 类型。
// 例如，如果 Parser 实现定义了自己的 options 结构体：
//
//	type customOptions struct {
//	    conf string
//	}
//
// 那么实现需要提供如下的 option 函数：
//
//	func WithConf(conf string) Option {
//	    return WrapImplSpecificOptFn(func(o *customOptions) {
//			o.conf = conf
//		}
//	}
func WrapImplSpecificOptFn[T any](optFn func(*T)) Option {
	return Option{
		implSpecificOptFn: optFn,
	}
}

// GetImplSpecificOptions provides Parser author the ability to extract their own custom options from the unified Option type.
// T: the type of the impl specific options struct.
// This function should be used within the Parser implementation's Transform function.
// It is recommended to provide a base T as the first argument, within which the Parser author can provide default values for the impl specific options.
//
// GetImplSpecificOptions 为 Parser 作者提供了从统一的 Option 类型中提取其自己的自定义选项的能力。
// T：特定于实现的 options 结构体的类型。
// 此函数应在 Parser 实现的 Transform 函数中使用。
// 建议提供一个 base T 作为第一个参数，Parser 作者可以在其中为特定于实现的选项提供默认值。
func GetImplSpecificOptions[T any](base *T, opts ...Option) *T {
	if base == nil {
		base = new(T)
	}

	for i := range opts {
		opt := opts[i]
		if opt.implSpecificOptFn != nil {
			s, ok := opt.implSpecificOptFn.(func(*T))
			if ok {
				s(base)
			}
		}
	}

	return base
}
