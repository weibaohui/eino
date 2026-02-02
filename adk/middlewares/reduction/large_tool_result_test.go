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

package reduction

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk/filesystem"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// mockBackend is a simple in-memory backend for testing
// mockBackend 是一个用于测试的简单内存后端
type mockBackend struct {
	files map[string]string
}

func newMockBackend() *mockBackend {
	return &mockBackend{
		files: make(map[string]string),
	}
}

func (m *mockBackend) Write(_ context.Context, wr *filesystem.WriteRequest) error {
	m.files[wr.FilePath] = wr.Content
	return nil
}

// TestToolResultOffloading_SmallResult tests that small tool results are not offloaded.
// TestToolResultOffloading_SmallResult 测试小工具结果不会被卸载。
func TestToolResultOffloading_SmallResult(t *testing.T) {
	ctx := context.Background()
	backend := newMockBackend()

	config := &toolResultOffloadingConfig{
		Backend:    backend,
		TokenLimit: 100, // Small limit for testing
	}

	middleware := newToolResultOffloading(ctx, config)

	// Create a mock endpoint that returns a small result
	// 创建一个返回小结果的模拟端点
	smallResult := "This is a small result"
	mockEndpoint := func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
		return &compose.ToolOutput{Result: smallResult}, nil
	}

	// Wrap the endpoint with the middleware
	// 用中间件包装端点
	wrappedEndpoint := middleware.Invokable(mockEndpoint)

	// Execute
	input := &compose.ToolInput{
		Name:   "test_tool",
		CallID: "call_123",
	}
	output, err := wrappedEndpoint(ctx, input)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Small result should pass through unchanged
	// 小结果应保持不变
	if output.Result != smallResult {
		t.Errorf("expected result %q, got %q", smallResult, output.Result)
	}

	// No file should be written
	// 不应写入任何文件
	if len(backend.files) != 0 {
		t.Errorf("expected no files to be written, got %d files", len(backend.files))
	}
}

// TestToolResultOffloading_LargeResult tests the offloading logic for large tool results.
// TestToolResultOffloading_LargeResult 测试大工具结果的 Offloading 逻辑。
func TestToolResultOffloading_LargeResult(t *testing.T) {
	ctx := context.Background()
	backend := newMockBackend()

	config := &toolResultOffloadingConfig{
		Backend:    backend,
		TokenLimit: 10, // Set very small limit to trigger offloading
	}

	middleware := newToolResultOffloading(ctx, config)

	// Create a large result (exceeds 10 * 4 = 40 bytes)
	// 创建一个大结果（超过 10 * 4 = 40 字节）
	largeResult := strings.Repeat("This is a long line of text that will exceed the token limit.\n", 10)
	mockEndpoint := func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
		return &compose.ToolOutput{Result: largeResult}, nil
	}

	wrappedEndpoint := middleware.Invokable(mockEndpoint)

	input := &compose.ToolInput{
		Name:   "test_tool",
		CallID: "call_456",
	}
	output, err := wrappedEndpoint(ctx, input)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Result should be replaced with a hint message
	// 结果应该被替换为提示信息
	if !strings.Contains(output.Result, "Tool result too large") {
		t.Errorf("expected result to contain 'Tool result too large', got %q", output.Result)
	}

	if !strings.Contains(output.Result, "call_456") {
		t.Errorf("expected result to contain call ID 'call_456', got %q", output.Result)
	}

	if !strings.Contains(output.Result, "/large_tool_result/call_456") {
		t.Errorf("expected result to contain file path, got %q", output.Result)
	}

	// A file should be written
	// 应该有一个文件被写入
	if len(backend.files) != 1 {
		t.Fatalf("expected 1 file to be written, got %d files", len(backend.files))
	}

	savedContent, ok := backend.files["/large_tool_result/call_456"]
	if !ok {
		t.Fatalf("expected file at /large_tool_result/call_456, got files: %v", backend.files)
	}

	if savedContent != largeResult {
		t.Errorf("saved content doesn't match original result")
	}
}

// TestToolResultOffloading_CustomPathGenerator tests custom path generator.
// TestToolResultOffloading_CustomPathGenerator 测试自定义路径生成器。
func TestToolResultOffloading_CustomPathGenerator(t *testing.T) {
	ctx := context.Background()
	backend := newMockBackend()

	customPath := "/custom/path/result.txt"
	config := &toolResultOffloadingConfig{
		Backend:    backend,
		TokenLimit: 10,
		PathGenerator: func(ctx context.Context, input *compose.ToolInput) (string, error) {
			return customPath, nil
		},
	}

	middleware := newToolResultOffloading(ctx, config)

	largeResult := strings.Repeat("Large content ", 100)
	mockEndpoint := func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
		return &compose.ToolOutput{Result: largeResult}, nil
	}

	wrappedEndpoint := middleware.Invokable(mockEndpoint)

	input := &compose.ToolInput{
		Name:   "test_tool",
		CallID: "call_789",
	}
	output, err := wrappedEndpoint(ctx, input)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Result should contain custom path
	// 结果应该包含自定义路径
	if !strings.Contains(output.Result, customPath) {
		t.Errorf("expected result to contain custom path %q, got %q", customPath, output.Result)
	}

	// File should be written to custom path
	// 文件应该写入到自定义路径
	if len(backend.files) != 1 {
		t.Fatalf("expected 1 file to be written, got %d files", len(backend.files))
	}

	if _, ok := backend.files[customPath]; !ok {
		t.Fatalf("expected file at %s, got files: %v", customPath, backend.files)
	}
}

func TestToolResultOffloading_PathGeneratorError(t *testing.T) {
	ctx := context.Background()
	backend := newMockBackend()

	expectedErr := errors.New("path generation failed")
	config := &toolResultOffloadingConfig{
		Backend:    backend,
		TokenLimit: 10,
		PathGenerator: func(ctx context.Context, input *compose.ToolInput) (string, error) {
			return "", expectedErr
		},
	}

	middleware := newToolResultOffloading(ctx, config)

	largeResult := strings.Repeat("Large content ", 100)
	mockEndpoint := func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
		return &compose.ToolOutput{Result: largeResult}, nil
	}

	wrappedEndpoint := middleware.Invokable(mockEndpoint)

	input := &compose.ToolInput{
		Name:   "test_tool",
		CallID: "call_error",
	}
	_, err := wrappedEndpoint(ctx, input)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestToolResultOffloading_EndpointError(t *testing.T) {
	ctx := context.Background()
	backend := newMockBackend()

	config := &toolResultOffloadingConfig{
		Backend:    backend,
		TokenLimit: 100,
	}

	middleware := newToolResultOffloading(ctx, config)

	expectedErr := errors.New("endpoint execution failed")
	mockEndpoint := func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
		return nil, expectedErr
	}

	wrappedEndpoint := middleware.Invokable(mockEndpoint)

	input := &compose.ToolInput{
		Name:   "test_tool",
		CallID: "call_endpoint_error",
	}
	_, err := wrappedEndpoint(ctx, input)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestToolResultOffloading_DefaultTokenLimit(t *testing.T) {
	ctx := context.Background()
	backend := newMockBackend()

	config := &toolResultOffloadingConfig{
		Backend:    backend,
		TokenLimit: 0, // Should default to 20000
	}

	middleware := newToolResultOffloading(ctx, config)

	// Create a result smaller than 20000 * 4 = 80000 bytes
	// 创建一个小于 20000 * 4 = 80000 字节的结果
	smallResult := strings.Repeat("x", 1000)
	mockEndpoint := func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
		return &compose.ToolOutput{Result: smallResult}, nil
	}

	wrappedEndpoint := middleware.Invokable(mockEndpoint)

	input := &compose.ToolInput{
		Name:   "test_tool",
		CallID: "call_default",
	}
	output, err := wrappedEndpoint(ctx, input)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should pass through unchanged
	// 应保持不变
	if output.Result != smallResult {
		t.Errorf("expected result to pass through unchanged")
	}

	// No file should be written
	// 不应写入任何文件
	if len(backend.files) != 0 {
		t.Errorf("expected no files to be written, got %d files", len(backend.files))
	}
}

func TestToolResultOffloading_Stream(t *testing.T) {
	ctx := context.Background()
	backend := newMockBackend()

	config := &toolResultOffloadingConfig{
		Backend:    backend,
		TokenLimit: 10,
	}

	middleware := newToolResultOffloading(ctx, config)

	// Create a streaming endpoint that returns large content
	// 创建一个返回大内容的流式端点
	largeResult := strings.Repeat("Large streaming content ", 100)
	mockStreamEndpoint := func(ctx context.Context, input *compose.ToolInput) (*compose.StreamToolOutput, error) {
		// Split the result into chunks
		chunks := []string{largeResult[:len(largeResult)/2], largeResult[len(largeResult)/2:]}
		return &compose.StreamToolOutput{
			Result: schema.StreamReaderFromArray(chunks),
		}, nil
	}

	wrappedEndpoint := middleware.Streamable(mockStreamEndpoint)

	input := &compose.ToolInput{
		Name:   "test_tool",
		CallID: "call_stream",
	}
	output, err := wrappedEndpoint(ctx, input)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Read the stream
	// 读取流
	var result strings.Builder
	for {
		chunk, err := output.Result.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("error reading stream: %v", err)
		}
		result.WriteString(chunk)
	}

	resultStr := result.String()

	// Result should be replaced with a message
	// 结果应被替换为消息
	if !strings.Contains(resultStr, "Tool result too large") {
		t.Errorf("expected result to contain 'Tool result too large', got %q", resultStr)
	}

	if !strings.Contains(resultStr, "call_stream") {
		t.Errorf("expected result to contain call ID 'call_stream', got %q", resultStr)
	}

	// File should be written
	// 文件应被写入
	if len(backend.files) != 1 {
		t.Fatalf("expected 1 file to be written, got %d files", len(backend.files))
	}

	savedContent, ok := backend.files["/large_tool_result/call_stream"]
	if !ok {
		t.Fatalf("expected file at /large_tool_result/call_stream, got files: %v", backend.files)
	}

	if savedContent != largeResult {
		t.Errorf("saved content doesn't match original result")
	}
}

func TestToolResultOffloading_StreamError(t *testing.T) {
	ctx := context.Background()
	backend := newMockBackend()

	config := &toolResultOffloadingConfig{
		Backend:    backend,
		TokenLimit: 10,
	}

	middleware := newToolResultOffloading(ctx, config)

	expectedErr := errors.New("stream endpoint failed")
	mockStreamEndpoint := func(ctx context.Context, input *compose.ToolInput) (*compose.StreamToolOutput, error) {
		return nil, expectedErr
	}

	wrappedEndpoint := middleware.Streamable(mockStreamEndpoint)

	input := &compose.ToolInput{
		Name:   "test_tool",
		CallID: "call_stream_error",
	}
	_, err := wrappedEndpoint(ctx, input)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestFormatToolMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single line",
			input:    "single line",
			expected: "1: single line\n",
		},
		{
			name:     "multiple lines",
			input:    "line1\nline2\nline3",
			expected: "1: line1\n2: line2\n3: line3\n",
		},
		{
			name:     "more than 10 lines",
			input:    "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12",
			expected: "1: 1\n2: 2\n3: 3\n4: 4\n5: 5\n6: 6\n7: 7\n8: 8\n9: 9\n10: 10\n",
		},
		{
			name:     "long line truncation",
			input:    strings.Repeat("a", 1500),
			expected: fmt.Sprintf("1: %s\n", strings.Repeat("a", 1000)),
		},
		{
			name:     "unicode characters",
			input:    "你好世界\n测试",
			expected: "1: 你好世界\n2: 测试\n",
		},
		{
			name:     "long unicode line",
			input:    strings.Repeat("你", 1500),
			expected: fmt.Sprintf("1: %s\n", strings.Repeat("你", 1000)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatToolMessage(tt.input)
			if result != tt.expected {
				t.Errorf("formatToolMessage() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestConcatString(t *testing.T) {
	tests := []struct {
		name        string
		chunks      []string
		expected    string
		expectError bool
	}{
		{
			name:     "single chunk",
			chunks:   []string{"hello"},
			expected: "hello",
		},
		{
			name:     "multiple chunks",
			chunks:   []string{"hello", " ", "world"},
			expected: "hello world",
		},
		{
			name:     "empty chunks",
			chunks:   []string{"", "", ""},
			expected: "",
		},
		{
			name:     "mixed chunks",
			chunks:   []string{"a", "", "b", "c"},
			expected: "abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr := schema.StreamReaderFromArray(tt.chunks)
			result, err := concatString(sr)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("concatString() = %q, want %q", result, tt.expected)
			}
		})
	}

	// Test nil stream
	t.Run("nil stream", func(t *testing.T) {
		_, err := concatString(nil)
		if err == nil {
			t.Error("expected error for nil stream, got nil")
		}
		if !strings.Contains(err.Error(), "stream is nil") {
			t.Errorf("expected 'stream is nil' error, got %v", err)
		}
	})
}

func TestToolResultOffloading_BackendWriteError(t *testing.T) {
	ctx := context.Background()

	// Create a backend that fails on write
	// 创建一个写入失败的后端
	backend := &failingBackend{
		writeErr: errors.New("write failed"),
	}

	config := &toolResultOffloadingConfig{
		Backend:    backend,
		TokenLimit: 10,
	}

	middleware := newToolResultOffloading(ctx, config)

	largeResult := strings.Repeat("Large content ", 100)
	mockEndpoint := func(ctx context.Context, input *compose.ToolInput) (*compose.ToolOutput, error) {
		return &compose.ToolOutput{Result: largeResult}, nil
	}

	wrappedEndpoint := middleware.Invokable(mockEndpoint)

	input := &compose.ToolInput{
		Name:   "test_tool",
		CallID: "call_write_error",
	}
	_, err := wrappedEndpoint(ctx, input)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "write failed") {
		t.Errorf("expected 'write failed' error, got %v", err)
	}
}

// failingBackend is a mock backend that can be configured to fail
// failingBackend 是一个可以配置为失败的模拟后端
type failingBackend struct {
	writeErr error
}

func (f *failingBackend) Write(context.Context, *filesystem.WriteRequest) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	return nil
}
