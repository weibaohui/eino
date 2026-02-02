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

// Package filesystem provides middlewares.
package filesystem

import (
	"context"

	"github.com/cloudwego/eino/adk/filesystem"
)

// FileInfo 定义文件信息结构
// 包含文件名、大小、修改时间等基本属性
type FileInfo = filesystem.FileInfo

// GrepMatch 定义 Grep 搜索匹配结果结构
// 包含匹配的文件路径、行号和行内容
type GrepMatch = filesystem.GrepMatch

// LsInfoRequest 定义列出目录信息的请求参数
type LsInfoRequest = filesystem.LsInfoRequest

// ReadRequest 定义读取文件的请求参数
type ReadRequest = filesystem.ReadRequest

// GrepRequest 定义 Grep 搜索的请求参数
type GrepRequest = filesystem.GrepRequest

// GlobInfoRequest 定义 Glob 模式匹配文件的请求参数
type GlobInfoRequest = filesystem.GlobInfoRequest

// WriteRequest 定义写入文件的请求参数
type WriteRequest = filesystem.WriteRequest

// EditRequest 定义编辑文件的请求参数
type EditRequest = filesystem.EditRequest

// Backend 定义文件系统操作的后端接口
// 该接口抽象了底层文件系统的具体实现，使得中间件可以支持本地文件系统、内存文件系统或其他远程文件系统。
// 实现该接口需要提供基本的文件操作能力，如列出目录、读取、写入、搜索和编辑文件。
type Backend interface {
	// LsInfo 列出指定目录下的文件信息
	// ctx: 上下文，用于控制超时和取消
	// req: 包含目录路径等请求参数
	// 返回: 文件信息列表或错误
	LsInfo(ctx context.Context, req *LsInfoRequest) ([]FileInfo, error)

	// Read 读取指定文件的内容
	// ctx: 上下文
	// req: 包含文件路径、读取范围等请求参数
	// 返回: 文件内容字符串或错误
	Read(ctx context.Context, req *ReadRequest) (string, error)

	// GrepRaw 在文件中搜索匹配指定模式的内容
	// ctx: 上下文
	// req: 包含搜索模式、文件范围等请求参数
	// 返回: 匹配结果列表或错误
	GrepRaw(ctx context.Context, req *GrepRequest) ([]GrepMatch, error)

	// GlobInfo 根据 Glob 模式查找文件信息
	// ctx: 上下文
	// req: 包含 Glob 模式等请求参数
	// 返回: 匹配的文件信息列表或错误
	GlobInfo(ctx context.Context, req *GlobInfoRequest) ([]FileInfo, error)

	// Write 将内容写入指定文件
	// ctx: 上下文
	// req: 包含文件路径、写入内容等请求参数
	// 返回: 错误信息
	Write(ctx context.Context, req *WriteRequest) error

	// Edit 编辑指定文件的内容
	// ctx: 上下文
	// req: 包含文件路径、编辑操作（如替换）等请求参数
	// 返回: 错误信息
	Edit(ctx context.Context, req *EditRequest) error
}
