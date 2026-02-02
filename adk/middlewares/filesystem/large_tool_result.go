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

package filesystem

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/slongfield/pyfmt"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// toolResultOffloadingConfig 定义工具结果卸载的配置。
// 用于配置如何将过大的工具执行结果保存到文件系统，而不是直接返回给模型。
type toolResultOffloadingConfig struct {
	// Backend 文件系统后端，用于存储过大的工具结果。
	// 必填。
	Backend Backend
	// TokenLimit 触发卸载的 Token 数量阈值。
	// 当结果的 Token 估算值超过此阈值时，将触发卸载。
	// 可选，默认为 20000。
	TokenLimit int
	// PathGenerator 生成卸载文件路径的函数。
	// 根据上下文和工具输入生成唯一的文件路径。
	// 可选，默认为 "/large_tool_result/{ToolCallID}"。
	PathGenerator func(ctx context.Context, input *compose.ToolInput) (string, error)
}

// toolResultOffloading 实现工具结果卸载的中间件结构。
// 当工具执行结果过大时，将结果写入文件系统，并返回文件路径及提示信息，
// 避免 Agent 上下文过长导致 Token 溢出或模型性能下降。
type toolResultOffloading struct {
	backend       Backend
	tokenLimit    int
	pathGenerator func(ctx context.Context, input *compose.ToolInput) (string, error)
}

// newToolResultOffloading 创建一个新的工具结果卸载中间件。
// 初始化配置，设置默认的 TokenLimit (20000) 和 PathGenerator。
//
// 参数:
//   - ctx: 上下文。
//   - config: 中间件配置。
//
// 返回:
//   - compose.ToolMiddleware: 构建好的工具中间件。
func newToolResultOffloading(ctx context.Context, config *toolResultOffloadingConfig) compose.ToolMiddleware {
	offloading := &toolResultOffloading{
		backend:       config.Backend,
		tokenLimit:    config.TokenLimit,
		pathGenerator: config.PathGenerator,
	}

	if offloading.tokenLimit == 0 {
		offloading.tokenLimit = 20000
	}

	if offloading.pathGenerator == nil {
		offloading.pathGenerator = func(ctx context.Context, input *compose.ToolInput) (string, error) {
			return fmt.Sprintf("/large_tool_result/%s", input.CallID), nil
		}
	}

	return compose.ToolMiddleware{
		Invokable:  offloading.invoke,
		Streamable: offloading.stream,
	}
}

// invoke 处理同步工具调用
// 执行原始工具调用，检查结果长度，如果超过阈值则进行卸载处理。
func (t *toolResultOffloading) invoke(endpoint compose.InvokableToolEndpoint) compose.InvokableToolEndpoint {
	return func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
		output, err := endpoint(ctx, input)
		if err != nil {
			return nil, err
		}
		result, err := t.handleResult(ctx, output.Result, input)
		if err != nil {
			return nil, err
		}
		return &compose.ToolOutput{Result: result}, nil
	}
}

// stream 处理流式工具调用。
// 目前流式调用会先聚合结果，然后进行卸载检查。
// 如果结果被卸载，将返回一个新的流，其中包含卸载提示信息。
func (t *toolResultOffloading) stream(endpoint compose.StreamableToolEndpoint) compose.StreamableToolEndpoint {
	return func(ctx context.Context, input *compose.ToolInput) (*compose.StreamToolOutput, error) {
		output, err := endpoint(ctx, input)
		if err != nil {
			return nil, err
		}
		result, err := concatString(output.Result)
		if err != nil {
			return nil, err
		}
		result, err = t.handleResult(ctx, result, input)
		if err != nil {
			return nil, err
		}
		return &compose.StreamToolOutput{Result: schema.StreamReaderFromArray([]string{result})}, nil
	}
}

// handleResult 处理工具执行结果
// 检查结果长度，如果超过阈值，则将结果写入文件并返回提示信息
func (t *toolResultOffloading) handleResult(ctx context.Context, result string, input *compose.ToolInput) (string, error) {
	if len(result) > t.tokenLimit*4 {
		path, err := t.pathGenerator(ctx, input)
		if err != nil {
			return "", err
		}

		nResult := formatToolMessage(result)
		nResult, err = pyfmt.Fmt(tooLargeToolMessage, map[string]any{
			"tool_call_id":   input.CallID,
			"file_path":      path,
			"content_sample": nResult,
		})
		if err != nil {
			return "", err
		}

		err = t.backend.Write(ctx, &WriteRequest{
			FilePath: path,
			Content:  result,
		})
		if err != nil {
			return "", err
		}

		return nResult, nil
	}

	return result, nil
}

func concatString(sr *schema.StreamReader[string]) (string, error) {
	if sr == nil {
		return "", errors.New("stream is nil")
	}
	sb := strings.Builder{}
	for {
		str, err := sr.Recv()
		if errors.Is(err, io.EOF) {
			return sb.String(), nil
		}
		if err != nil {
			return "", err
		}
		sb.WriteString(str)
	}
}

// formatToolMessage formats the tool message for preview (first 10 lines, max 1000 chars per line).
// formatToolMessage 格式化工具消息以便预览（前 10 行，每行最多 1000 个字符）。
func formatToolMessage(s string) string {
	reader := bufio.NewScanner(strings.NewReader(s))
	var b strings.Builder

	lineNum := 1
	for reader.Scan() {
		if lineNum > 10 {
			break
		}
		line := reader.Text()

		if utf8.RuneCountInString(line) > 1000 {
			runes := []rune(line)
			line = string(runes[:1000])
		}

		b.WriteString(fmt.Sprintf("%d: %s\n", lineNum, line))

		lineNum++
	}

	return b.String()
}
