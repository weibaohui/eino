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

const (
	docMetaDataKeySubIndexes   = "_sub_indexes"
	docMetaDataKeyScore        = "_score"
	docMetaDataKeyExtraInfo    = "_extra_info"
	docMetaDataKeyDSL          = "_dsl"
	docMetaDataKeyDenseVector  = "_dense_vector"
	docMetaDataKeySparseVector = "_sparse_vector"
)

// Document is a piece of text with metadata.
//
// Document 是带有元数据的文本片段。
type Document struct {
	// ID is the unique identifier of the document.
	// ID 是文档的唯一标识符。
	ID string `json:"id"`
	// Content is the content of the document.
	// Content 是文档的内容。
	Content string `json:"content"`
	// MetaData is the metadata of the document, can be used to store extra information.
	// MetaData 是文档的元数据，可用于存储额外信息。
	MetaData map[string]any `json:"meta_data"`
}

// String returns the content of the document.
//
// String 返回文档的内容。
func (d *Document) String() string {
	return d.Content
}

// WithSubIndexes sets the sub indexes of the document.
// can use doc.SubIndexes() to get the sub indexes, useful for search engine to use sub indexes to search.
//
// WithSubIndexes 设置文档的子索引。
// 可以使用 doc.SubIndexes() 获取子索引，这对于搜索引擎使用子索引进行搜索非常有用。
func (d *Document) WithSubIndexes(indexes []string) *Document {
	if d.MetaData == nil {
		d.MetaData = make(map[string]any)
	}

	d.MetaData[docMetaDataKeySubIndexes] = indexes

	return d
}

// SubIndexes returns the sub indexes of the document.
// can use doc.WithSubIndexes() to set the sub indexes.
//
// SubIndexes 返回文档的子索引。
// 可以使用 doc.WithSubIndexes() 设置子索引。
func (d *Document) SubIndexes() []string {
	if d.MetaData == nil {
		return nil
	}

	indexes, ok := d.MetaData[docMetaDataKeySubIndexes].([]string)
	if ok {
		return indexes
	}

	return nil
}

// WithScore sets the score of the document.
// can use doc.Score() to get the score.
//
// WithScore 设置文档的分数。
// 可以使用 doc.Score() 获取分数。
func (d *Document) WithScore(score float64) *Document {
	if d.MetaData == nil {
		d.MetaData = make(map[string]any)
	}

	d.MetaData[docMetaDataKeyScore] = score

	return d
}

// Score returns the score of the document.
// can use doc.WithScore() to set the score.
//
// Score 返回文档的分数。
// 可以使用 doc.WithScore() 设置分数。
func (d *Document) Score() float64 {
	if d.MetaData == nil {
		return 0
	}

	score, ok := d.MetaData[docMetaDataKeyScore].(float64)
	if ok {
		return score
	}

	return 0
}

// WithExtraInfo sets the extra info of the document.
// can use doc.ExtraInfo() to get the extra info.
//
// WithExtraInfo 设置文档的额外信息。
// 可以使用 doc.ExtraInfo() 获取额外信息。
func (d *Document) WithExtraInfo(extraInfo string) *Document {
	if d.MetaData == nil {
		d.MetaData = make(map[string]any)
	}

	d.MetaData[docMetaDataKeyExtraInfo] = extraInfo

	return d
}

// ExtraInfo returns the extra info of the document.
// can use doc.WithExtraInfo() to set the extra info.
//
// ExtraInfo 返回文档的额外信息。
// 可以使用 doc.WithExtraInfo() 设置额外信息。
func (d *Document) ExtraInfo() string {
	if d.MetaData == nil {
		return ""
	}

	extraInfo, ok := d.MetaData[docMetaDataKeyExtraInfo].(string)
	if ok {
		return extraInfo
	}

	return ""
}

// WithDSLInfo sets the dsl info of the document.
// can use doc.DSLInfo() to get the dsl info.
//
// WithDSLInfo 设置文档的 DSL 信息。
// 可以使用 doc.DSLInfo() 获取 DSL 信息。
func (d *Document) WithDSLInfo(dslInfo map[string]any) *Document {
	if d.MetaData == nil {
		d.MetaData = make(map[string]any)
	}

	d.MetaData[docMetaDataKeyDSL] = dslInfo

	return d
}

// DSLInfo returns the dsl info of the document.
// can use doc.WithDSLInfo() to set the dsl info.
//
// DSLInfo 返回文档的 DSL 信息。
// 可以使用 doc.WithDSLInfo() 设置 DSL 信息。
func (d *Document) DSLInfo() map[string]any {
	if d.MetaData == nil {
		return nil
	}

	dslInfo, ok := d.MetaData[docMetaDataKeyDSL].(map[string]any)
	if ok {
		return dslInfo
	}

	return nil
}

// WithDenseVector sets the dense vector of the document.
// can use doc.DenseVector() to get the dense vector.
//
// WithDenseVector 设置文档的稠密向量。
// 可以使用 doc.DenseVector() 获取稠密向量。
func (d *Document) WithDenseVector(vector []float64) *Document {
	if d.MetaData == nil {
		d.MetaData = make(map[string]any)
	}

	d.MetaData[docMetaDataKeyDenseVector] = vector

	return d
}

// DenseVector returns the dense vector of the document.
// can use doc.WithDenseVector() to set the dense vector.
//
// DenseVector 返回文档的稠密向量。
// 可以使用 doc.WithDenseVector() 设置稠密向量。
func (d *Document) DenseVector() []float64 {
	if d.MetaData == nil {
		return nil
	}

	vector, ok := d.MetaData[docMetaDataKeyDenseVector].([]float64)
	if ok {
		return vector
	}

	return nil
}

// WithSparseVector sets the sparse vector of the document, key indices -> value vector.
// can use doc.SparseVector() to get the sparse vector.
//
// WithSparseVector 设置文档的稀疏向量，键索引 -> 值向量。
// 可以使用 doc.SparseVector() 获取稀疏向量。
func (d *Document) WithSparseVector(sparse map[int]float64) *Document {
	if d.MetaData == nil {
		d.MetaData = make(map[string]any)
	}

	d.MetaData[docMetaDataKeySparseVector] = sparse

	return d
}

// SparseVector returns the sparse vector of the document, key indices -> value vector.
// can use doc.WithSparseVector() to set the sparse vector.
//
// SparseVector 返回文档的稀疏向量，键索引 -> 值向量。
// 可以使用 doc.WithSparseVector() 设置稀疏向量。
func (d *Document) SparseVector() map[int]float64 {
	if d.MetaData == nil {
		return nil
	}

	sparse, ok := d.MetaData[docMetaDataKeySparseVector].(map[int]float64)
	if ok {
		return sparse
	}

	return nil
}
