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

package model

import "github.com/cloudwego/eino/schema"

// Options is the common options for the model.
//
// Options 是模型的通用选项。
type Options struct {
	// Temperature is the temperature for the model, which controls the randomness of the model.
	// Temperature 是模型的温度，控制模型的随机性。
	Temperature *float32
	// MaxTokens is the max number of tokens, if reached the max tokens, the model will stop generating, and mostly return an finish reason of "length".
	// MaxTokens 是最大 token 数，如果达到最大 token 数，模型将停止生成，并且通常返回 "length" 的结束原因。
	MaxTokens *int
	// Model is the model name.
	// Model 是模型名称。
	Model *string
	// TopP is the top p for the model, which controls the diversity of the model.
	// TopP 是模型的 top p，控制模型的多样性。
	TopP *float32
	// Stop is the stop words for the model, which controls the stopping condition of the model.
	// Stop 是模型的停止词，控制模型的停止条件。
	Stop []string
	// Tools is a list of tools the model may call.
	// Tools 是模型可以调用的工具列表。
	Tools []*schema.ToolInfo
	// ToolChoice controls which tool is called by the model.
	// ToolChoice 控制模型调用哪个工具。
	ToolChoice *schema.ToolChoice
	// AllowedToolNames specifies a list of tool names that the model is allowed to call.
	// This allows for constraining the model to a specific subset of the available tools.
	// AllowedToolNames 指定模型允许调用的工具名称列表。
	// 这允许将模型限制为可用工具的特定子集。
	AllowedToolNames []string
}

// Option is the call option for ChatModel component.
//
// Option 是 ChatModel 组件的调用选项。
type Option struct {
	apply func(opts *Options)

	implSpecificOptFn any
}

// WithTemperature is the option to set the temperature for the model.
//
// WithTemperature 是设置模型温度的选项。
func WithTemperature(temperature float32) Option {
	return Option{
		apply: func(opts *Options) {
			opts.Temperature = &temperature
		},
	}
}

// WithMaxTokens is the option to set the max tokens for the model.
//
// WithMaxTokens 是设置模型最大 token 数的选项。
func WithMaxTokens(maxTokens int) Option {
	return Option{
		apply: func(opts *Options) {
			opts.MaxTokens = &maxTokens
		},
	}
}

// WithModel is the option to set the model name.
//
// WithModel 是设置模型名称的选项。
func WithModel(name string) Option {
	return Option{
		apply: func(opts *Options) {
			opts.Model = &name
		},
	}
}

// WithTopP is the option to set the top p for the model.
//
// WithTopP 是设置模型 top p 的选项。
func WithTopP(topP float32) Option {
	return Option{
		apply: func(opts *Options) {
			opts.TopP = &topP
		},
	}
}

// WithStop is the option to set the stop words for the model.
//
// WithStop 是设置模型停止词的选项。
func WithStop(stop []string) Option {
	return Option{
		apply: func(opts *Options) {
			opts.Stop = stop
		},
	}
}

// WithTools is the option to set tools for the model.
//
// WithTools 是设置模型工具的选项。
func WithTools(tools []*schema.ToolInfo) Option {
	if tools == nil {
		tools = []*schema.ToolInfo{}
	}
	return Option{
		apply: func(opts *Options) {
			opts.Tools = tools
		},
	}
}

// WithToolChoice sets the tool choice for the model. It also allows for providing a list of
// tool names to constrain the model to a specific subset of the available tools.
func WithToolChoice(toolChoice schema.ToolChoice, allowedToolNames ...string) Option {
	return Option{
		apply: func(opts *Options) {
			opts.ToolChoice = &toolChoice
			opts.AllowedToolNames = allowedToolNames
		},
	}
}

// WrapImplSpecificOptFn is the option to wrap the implementation specific option function.
func WrapImplSpecificOptFn[T any](optFn func(*T)) Option {
	return Option{
		implSpecificOptFn: optFn,
	}
}

// GetCommonOptions extract model Options from Option list, optionally providing a base Options with default values.
// e.g.
//
//	modelOption := &model.Options{
//		Temperature: 0.5, // default value
//	}
//
//	modelOption := model.GetCommonOptions(modelOption, opts...)
//
// GetCommonOptions 从 Option 列表中提取 model Options，可选择提供带有默认值的 base Options。
// 例如：
//
//	modelOption := &model.Options{
//		Temperature: 0.5, // default value
//	}
//
//	modelOption := model.GetCommonOptions(modelOption, opts...)
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
 

// GetImplSpecificOptions extract the implementation specific options from Option list, optionally providing a base options with default values.
// e.g.
//
//	myOption := &MyOption{
//		Field1: "default_value",
//	}
//
//	myOption := model.GetImplSpecificOptions(myOption, opts...)
//
// GetImplSpecificOptions 从 Option 列表中提取特定于实现的选项，可选择提供带有默认值的 base options。
// 例如：
//
//	myOption := &MyOption{
//		Field1: "default_value",
//	}
//
//	myOption := model.GetImplSpecificOptions(myOption, opts...)
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
