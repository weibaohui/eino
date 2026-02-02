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

package indexer

import "github.com/cloudwego/eino/components/embedding"

// Options is the options for the indexer.
//
// Options 是 indexer 的选项。
type Options struct {
	// SubIndexes is the sub indexes to be indexed.
	// SubIndexes 是要索引的子索引。
	SubIndexes []string
	// Embedding is the embedding component.
	// Embedding 是 embedding 组件。
	Embedding embedding.Embedder
}

// WithSubIndexes is the option to set the sub indexes for the indexer.
//
// WithSubIndexes 是设置 indexer 子索引的选项。
func WithSubIndexes(subIndexes []string) Option {
	return Option{
		apply: func(opts *Options) {
			opts.SubIndexes = subIndexes
		},
	}
}

// WithEmbedding is the option to set the embedder for the indexer, which convert document to embeddings.
//
// WithEmbedding 是设置 indexer 的 embedder 的选项，它将文档转换为 embeddings。
func WithEmbedding(emb embedding.Embedder) Option {
	return Option{
		apply: func(opts *Options) {
			opts.Embedding = emb
		},
	}
}

// Option is the call option for Indexer component.
//
// Option 是 Indexer 组件的调用选项。
type Option struct {
	apply func(opts *Options)

	implSpecificOptFn any
}

// GetCommonOptions extract indexer Options from Option list, optionally providing a base Options with default values.
// e.g.
//
//	indexerOption := &IndexerOption{
//		SubIndexes: []string{"default_sub_index"}, // default value
//	}
//
//	indexerOption := indexer.GetCommonOptions(indexerOption, opts...)
//
// GetCommonOptions 从 Option 列表中提取 indexer Options，可选择提供带有默认值的 base Options。
// 例如：
//
//	indexerOption := &IndexerOption{
//		SubIndexes: []string{"default_sub_index"}, // default value
//	}
//
//	indexerOption := indexer.GetCommonOptions(indexerOption, opts...)
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

// WrapImplSpecificOptFn is the option to wrap the implementation specific option function.
//
// WrapImplSpecificOptFn 是包装特定于实现的选项函数的选项。
func WrapImplSpecificOptFn[T any](optFn func(*T)) Option {
	return Option{
		implSpecificOptFn: optFn,
	}
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
