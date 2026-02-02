/*
 * Copyright 2025 CloudWeGo Authors
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

// Package filesystem provides file system operations.
package filesystem

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// FileInfo represents basic file metadata information.
// FileInfo 表示基本的文件元数据信息。
type FileInfo struct {
	// Path is the absolute path of the file or directory.
	// Path 是文件或目录的绝对路径。
	Path string
}

// GrepMatch represents a single pattern match result.
// GrepMatch 表示单个模式匹配结果。
type GrepMatch struct {
	// Path is the absolute path of the file where the match occurred.
	// Path 是发生匹配的文件的绝对路径。
	Path string
	// Line is the 1-based line number of the match.
	// Line 是匹配项的行号（从 1 开始）。
	Line int
	// Content is the full text content of the line containing the match.
	// Content 是包含匹配项的行的全文内容。
	Content string
}

// LsInfoRequest contains parameters for listing file information.
// LsInfoRequest 包含用于列出文件信息的参数。
type LsInfoRequest struct {
	// Path specifies the absolute directory path to list.
	// It must be an absolute path starting with '/'.
	// An empty string is treated as the root directory ("/").
	// Path 指定要列出的绝对目录路径。
	// 它必须是以 '/' 开头的绝对路径。
	// 空字符串被视为根目录 ("/")。
	Path string
}

// ReadRequest contains parameters for reading file content.
// ReadRequest 包含用于读取文件内容的参数。
type ReadRequest struct {
	// FilePath is the absolute path to the file to be read. Must start with '/'.
	// FilePath 是要读取的文件的绝对路径。必须以 '/' 开头。
	FilePath string

	// Offset is the 0-based line number to start reading from.
	// If negative, it is treated as 0. Defaults to 0.
	// Offset 是开始读取的行号（从 0 开始）。
	// 如果为负数，则视为 0。默认为 0。
	Offset int

	// Limit specifies the maximum number of lines to read.
	// If non-positive (<= 0), a default limit is used (typically 200).
	// Limit 指定要读取的最大行数。
	// 如果非正数 (<= 0)，则使用默认限制（通常为 200）。
	Limit int
}

// GrepRequest contains parameters for searching file content.
// GrepRequest 包含用于搜索文件内容的参数。
type GrepRequest struct {
	// Pattern is the literal string to search for. This is not a regular expression.
	// The search performs an exact substring match within the file's content.
	// For example, "TODO" will match any line containing "TODO".
	// Pattern 是要搜索的文字字符串。这不是正则表达式。
	// 搜索在文件内容中执行精确的子字符串匹配。
	// 例如，"TODO" 将匹配任何包含 "TODO" 的行。
	Pattern string

	// Path is an optional directory path to limit the search scope.
	// If empty, the search is performed from the working directory.
	// Path 是一个可选的目录路径，用于限制搜索范围。
	// 如果为空，则从工作目录执行搜索。
	Path string

	// Glob is an optional pattern to filter the files to be searched.
	// It filters by file path, not content. If empty, no files are filtered.
	// Supports standard glob wildcards:
	//   - `*` matches any characters except path separators.
	//   - `**` matches any directories recursively.
	//   - `?` matches a single character.
	//   - `[abc]` matches one character from the set.
	// Glob 是一个可选模式，用于过滤要搜索的文件。
	// 它按文件路径过滤，而不是按内容过滤。如果为空，则不过滤任何文件。
	// 支持标准 glob 通配符：
	//   - `*` 匹配除路径分隔符之外的任何字符。
	//   - `**` 递归匹配任何目录。
	//   - `?` 匹配单个字符。
	//   - `[abc]` 匹配集合中的一个字符。
	Glob string
}

// GlobInfoRequest contains parameters for glob pattern matching.
// GlobInfoRequest 包含用于 glob 模式匹配的参数。
type GlobInfoRequest struct {
	// Pattern is the glob expression used to match file paths.
	// It supports standard glob syntax:
	//   - `*` matches any characters except path separators.
	//   - `**` matches any directories recursively.
	//   - `?` matches a single character.
	//   - `[abc]` matches one character from the set.
	// Pattern 是用于匹配文件路径的 glob 表达式。
	// 它支持标准 glob 语法：
	//   - `*` 匹配除路径分隔符之外的任何字符。
	//   - `**` 递归匹配任何目录。
	//   - `?` 匹配单个字符。
	//   - `[abc]` 匹配集合中的一个字符。
	Pattern string

	// Path is the base directory from which to start the search.
	// The glob pattern is applied relative to this path. Defaults to the root directory ("/").
	// Path 是开始搜索的基目录。
	// glob 模式是相对于此路径应用的。默认为根目录 ("/")。
	Path string
}

// WriteRequest contains parameters for writing file content.
// WriteRequest 包含用于写入文件内容的参数。
type WriteRequest struct {
	// FilePath is the absolute path of the file to write. Must start with '/'.
	// The file will be created if it does not exist, or error if file exists.
	// FilePath 是要写入的文件的绝对路径。必须以 '/' 开头。
	// 如果文件不存在，将创建该文件；如果文件存在，则报错。
	FilePath string

	// Content is the data to be written to the file.
	// Content 是要写入文件的数据。
	Content string
}

// EditRequest contains parameters for editing file content.
// EditRequest 包含用于编辑文件内容的参数。
type EditRequest struct {
	// FilePath is the absolute path of the file to edit. Must start with '/'.
	// FilePath 是要编辑的文件的绝对路径。必须以 '/' 开头。
	FilePath string

	// OldString is the exact string to be replaced. It must be non-empty and will be matched literally, including whitespace.
	// OldString 是要被替换的精确字符串。它必须非空，并且将进行字面匹配，包括空格。
	OldString string

	// NewString is the string that will replace OldString.
	// It must be different from OldString.
	// An empty string can be used to effectively delete OldString.
	// NewString 是将替换 OldString 的字符串。
	// 它必须与 OldString 不同。
	// 可以使用空字符串来有效地删除 OldString。
	NewString string

	// ReplaceAll controls the replacement behavior.
	// If true, all occurrences of OldString are replaced.
	// If false, the operation fails unless OldString appears exactly once in the file.
	// ReplaceAll 控制替换行为。
	// 如果为 true，则替换所有出现的 OldString。
	// 如果为 false，除非 OldString 在文件中恰好出现一次，否则操作失败。
	ReplaceAll bool
}

// Backend is a pluggable, unified file backend protocol interface.
//
// All methods use struct-based parameters to allow future extensibility
// without breaking backward compatibility.
// Backend 是一个可插拔的、统一的文件后端协议接口。
//
// 所有方法都使用基于结构的参数，以允许未来的可扩展性，
// 而不会破坏向后兼容性。
type Backend interface {
	// LsInfo lists file information under the given path.
	//
	// Returns:
	//   - []FileInfo: List of matching file information
	//   - error: Error if the operation fails
	// LsInfo 列出给定路径下的文件信息。
	//
	// 返回：
	//   - []FileInfo: 匹配的文件信息列表
	//   - error: 如果操作失败则返回错误
	LsInfo(ctx context.Context, req *LsInfoRequest) ([]FileInfo, error)

	// Read reads file content with support for line-based offset and limit.
	//
	// Returns:
	//   - string: The file content read
	//   - error: Error if file does not exist or read fails
	// Read 读取文件内容，支持基于行的偏移量和限制。
	//
	// 返回：
	//   - string: 读取的文件内容
	//   - error: 如果文件不存在或读取失败则返回错误
	Read(ctx context.Context, req *ReadRequest) (string, error)

	// GrepRaw searches for content matching the specified pattern in files.
	//
	// Returns:
	//   - []GrepMatch: List of all matching results
	//   - error: Error if the search fails
	// GrepRaw 在文件中搜索匹配指定模式的内容。
	//
	// 返回：
	//   - []GrepMatch: 所有匹配结果的列表
	//   - error: 如果搜索失败则返回错误
	GrepRaw(ctx context.Context, req *GrepRequest) ([]GrepMatch, error)

	// GlobInfo returns file information matching the glob pattern.
	//
	// Returns:
	//   - []FileInfo: List of matching file information
	//   - error: Error if the pattern is invalid or operation fails
	// GlobInfo 返回匹配 glob 模式的文件信息。
	//
	// 返回：
	//   - []FileInfo: 匹配的文件信息列表
	//   - error: 如果模式无效或操作失败则返回错误
	GlobInfo(ctx context.Context, req *GlobInfoRequest) ([]FileInfo, error)

	// Write creates or updates file content.
	//
	// Returns:
	//   - error: Error if the write operation fails
	// Write 创建或更新文件内容。
	//
	// 返回：
	//   - error: 如果写入操作失败则返回错误
	Write(ctx context.Context, req *WriteRequest) error

	// Edit replaces string occurrences in a file.
	//
	// Returns:
	//   - error: Error if file does not exist, OldString is empty, or OldString is not found
	// Edit 替换文件中出现的字符串。
	//
	// 返回：
	//   - error: 如果文件不存在、OldString 为空或未找到 OldString 则返回错误
	Edit(ctx context.Context, req *EditRequest) error
}

type ExecuteRequest struct {
	Command string
}

type ExecuteResponse struct {
	Output    string
	ExitCode  *int
	Truncated bool
}

type ShellBackend interface {
	Backend
	Execute(ctx context.Context, input *ExecuteRequest) (result *ExecuteResponse, err error)
}

type StreamingShellBackend interface {
	Backend
	ExecuteStreaming(ctx context.Context, input *ExecuteRequest) (result *schema.StreamReader[*ExecuteResponse], err error)
}
