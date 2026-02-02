package schema

// Document 表示一个文档，通常包含内容、ID 和元数据。
// 它用于索引和检索系统。
type Document struct {
	// ID 是文档的唯一标识符。
	ID string
	// Content 是文档的文本内容。
	Content string
	// MetaData 存储文档的额外信息，如作者、创建时间等。
	MetaData map[string]any
}

const (
	// docMetaDataKeySubIndexes 是元数据中存储子索引的键。
	docMetaDataKeySubIndexes = "_sub_indexes"
)

// SubIndexes 从文档的元数据中获取子索引。
// 如果不存在子索引，则返回 nil。
func (d *Document) SubIndexes() []string {
	if d.MetaData == nil {
		return nil
	}

	indexes, ok := d.MetaData[docMetaDataKeySubIndexes]
	if !ok {
		return nil
	}

	return indexes.([]string)
}

// WithSubIndexes 设置文档的子索引。
// 可以使用 doc.SubIndexes() 获取子索引，这对于搜索引擎使用子索引进行搜索非常有用。
func (d *Document) WithSubIndexes(indexes []string) *Document {
	if d.MetaData == nil {
		d.MetaData = make(map[string]any)
	}

	d.MetaData[docMetaDataKeySubIndexes] = indexes

	return d
}
