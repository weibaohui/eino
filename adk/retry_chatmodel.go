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

package adk

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"reflect"
	"time"

	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/internal/generic"
	"github.com/cloudwego/eino/schema"
)

var (
	// ErrExceedMaxRetries is returned when the maximum number of retries has been exceeded.
	// Use errors.Is to check if an error is due to max retries being exceeded:
	//
	//   if errors.Is(err, adk.ErrExceedMaxRetries) {
	//       // handle max retries exceeded
	//   }
	//
	// Use errors.As to extract the underlying RetryExhaustedError for the last error details:
	//
	//   var retryErr *adk.RetryExhaustedError
	//   if errors.As(err, &retryErr) {
	//       fmt.Printf("last error was: %v\n", retryErr.LastErr)
	//   }
	// ErrExceedMaxRetries 当超过最大重试次数时返回。
	// 使用 errors.Is 检查错误是否由于超过最大重试次数：
	//
	//   if errors.Is(err, adk.ErrExceedMaxRetries) {
	//       // 处理最大重试次数超限
	//   }
	//
	// 使用 errors.As 提取底层的 RetryExhaustedError 以获取最后一次错误的详细信息：
	//
	//   var retryErr *adk.RetryExhaustedError
	//   if errors.As(err, &retryErr) {
	//       fmt.Printf("last error was: %v\n", retryErr.LastErr)
	//   }
	ErrExceedMaxRetries = errors.New("exceeds max retries")
)

// RetryExhaustedError is returned when all retry attempts have been exhausted.
// It wraps the last error that occurred during retry attempts.
// RetryExhaustedError 当所有重试尝试都已耗尽时返回。
// 它包装了重试尝试期间发生的最后一个错误。
type RetryExhaustedError struct {
	LastErr      error
	TotalRetries int
}

func (e *RetryExhaustedError) Error() string {
	if e.LastErr != nil {
		return fmt.Sprintf("exceeds max retries: last error: %v", e.LastErr)
	}
	return "exceeds max retries"
}

func (e *RetryExhaustedError) Unwrap() error {
	return ErrExceedMaxRetries
}

// WillRetryError is emitted when a retryable error occurs and a retry will be attempted.
// It allows end-users to observe retry events in real-time via AgentEvent.
//
// Field design rationale:
//   - ErrStr (exported): Stores the error message string for Gob serialization during checkpointing.
//     This ensures the error message is preserved after checkpoint restore.
//   - err (unexported): Stores the original error for Unwrap() support at runtime.
//     This field is intentionally unexported because Gob serialization would fail for unregistered
//     concrete error types. Since end-users only need the original error when the AgentEvent first
//     occurs (not after restoring from checkpoint), skipping serialization is acceptable.
//     After checkpoint restore, err will be nil and Unwrap() returns nil.
//
// WillRetryError 当发生可重试错误并将尝试重试时发出。
// 它允许最终用户通过 AgentEvent 实时观察重试事件。
//
// 字段设计理由：
//   - ErrStr (导出): 存储错误消息字符串，用于 Checkpoint 期间的 Gob 序列化。
//     这确保了在 Checkpoint 恢复后错误消息得以保留。
//   - err (未导出): 存储原始错误，以便在运行时支持 Unwrap()。
//     该字段特意未导出，因为 Gob 序列化对于未注册的具体错误类型会失败。由于最终用户仅在 AgentEvent 首次发生时（而不是从 Checkpoint 恢复后）需要原始错误，
//     因此跳过序列化是可以接受的。
//     从 Checkpoint 恢复后，err 将为 nil，Unwrap() 返回 nil。
type WillRetryError struct {
	ErrStr       string
	RetryAttempt int
	err          error
}

func (e *WillRetryError) Error() string {
	return e.ErrStr
}

func (e *WillRetryError) Unwrap() error {
	return e.err
}

func init() {
	schema.RegisterName[*WillRetryError]("eino_adk_chatmodel_will_retry_error")
}

// ModelRetryConfig configures retry behavior for the ChatModel node.
// It defines how the agent should handle transient failures when calling the ChatModel.
// ModelRetryConfig 配置 ChatModel 节点的重试行为。
// 它定义了 Agent 在调用 ChatModel 时应如何处理暂时性故障。
type ModelRetryConfig struct {
	// MaxRetries specifies the maximum number of retry attempts.
	// A value of 0 means no retries will be attempted.
	// A value of 3 means up to 3 retry attempts (4 total calls including the initial attempt).
	// MaxRetries 指定最大重试尝试次数。
	// 值为 0 表示不尝试重试。
	// 值为 3 表示最多 3 次重试尝试（包括初始尝试在内共 4 次调用）。
	MaxRetries int

	// IsRetryAble is a function that determines whether an error should trigger a retry.
	// If nil, all errors are considered retry-able.
	// Return true if the error is transient and the operation should be retried.
	// Return false if the error is permanent and should be propagated immediately.
	// IsRetryAble 是一个函数，用于确定错误是否应触发重试。
	// 如果为 nil，则所有错误都被认为是可重试的。
	// 如果错误是暂时的且操作应重试，则返回 true。
	// 如果错误是永久性的且应立即传播，则返回 false。
	IsRetryAble func(ctx context.Context, err error) bool

	// BackoffFunc calculates the delay before the next retry attempt.
	// The attempt parameter starts at 1 for the first retry.
	// If nil, a default exponential backoff with jitter is used:
	// base delay 100ms, exponentially increasing up to 10s max,
	// with random jitter (0-50% of delay) to prevent thundering herd.
	// BackoffFunc 计算下一次重试尝试前的延迟。
	// 对于第一次重试，attempt 参数从 1 开始。
	// 如果为 nil，则使用默认的带有抖动的指数退避：
	// 基础延迟 100ms，指数增加最多到 10s，
	// 带有随机抖动（延迟的 0-50%）以防止惊群效应。
	BackoffFunc func(ctx context.Context, attempt int) time.Duration
}

func defaultIsRetryAble(_ context.Context, err error) bool {
	return err != nil
}

func defaultBackoff(_ context.Context, attempt int) time.Duration {
	baseDelay := 100 * time.Millisecond
	maxDelay := 10 * time.Second

	if attempt <= 0 {
		return baseDelay
	}

	if attempt > 7 {
		return maxDelay + time.Duration(rand.Int63n(int64(maxDelay/2)))
	}

	delay := baseDelay * time.Duration(1<<uint(attempt-1))
	if delay > maxDelay {
		delay = maxDelay
	}

	jitter := time.Duration(rand.Int63n(int64(delay / 2)))
	return delay + jitter
}

// genErrWrapper 生成一个错误包装函数。
// 该函数检查错误是否可重试，如果可重试且未达到最大重试次数，则返回 WillRetryError。
func genErrWrapper(ctx context.Context, config ModelRetryConfig, info streamRetryInfo) func(error) error {
	return func(err error) error {
		isRetryAble := config.IsRetryAble == nil || config.IsRetryAble(ctx, err)
		hasRetriesLeft := info.attempt < config.MaxRetries

		if isRetryAble && hasRetriesLeft {
			return &WillRetryError{ErrStr: err.Error(), RetryAttempt: info.attempt, err: err}
		}
		return err
	}
}

type retryChatModel struct {
	inner                 model.ToolCallingChatModel
	config                *ModelRetryConfig
	innerHandlesCallbacks bool
}

// newRetryChatModel 创建一个新的带有重试逻辑的 ChatModel。
func newRetryChatModel(inner model.ToolCallingChatModel, config *ModelRetryConfig) *retryChatModel {
	innerHandlesCallbacks := false
	if ch, ok := inner.(components.Checker); ok {
		innerHandlesCallbacks = ch.IsCallbacksEnabled()
	}
	return &retryChatModel{inner: inner, config: config, innerHandlesCallbacks: innerHandlesCallbacks}
}

// WithTools 为 retryChatModel 绑定工具。
// 它会同时为内部的 ChatModel 绑定工具，并返回一个新的 retryChatModel 实例。
func (r *retryChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	newInner, err := r.inner.WithTools(tools)
	if err != nil {
		return nil, err
	}
	innerHandlesCallbacks := false
	if ch, ok := newInner.(components.Checker); ok {
		innerHandlesCallbacks = ch.IsCallbacksEnabled()
	}
	return &retryChatModel{inner: newInner, config: r.config, innerHandlesCallbacks: innerHandlesCallbacks}, nil
}

func (r *retryChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	isRetryAble := r.config.IsRetryAble
	if isRetryAble == nil {
		isRetryAble = defaultIsRetryAble
	}
	backoffFunc := r.config.BackoffFunc
	if backoffFunc == nil {
		backoffFunc = defaultBackoff
	}

	var lastErr error
	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		var out *schema.Message
		var err error

		if r.innerHandlesCallbacks {
			out, err = r.inner.Generate(ctx, input, opts...)
		} else {
			out, err = r.generateWithProxyCallbacks(ctx, input, opts...)
		}

		if err == nil {
			return out, nil
		}

		if !isRetryAble(ctx, err) {
			return nil, err
		}

		lastErr = err
		if attempt < r.config.MaxRetries {
			log.Printf("retrying ChatModel.Generate (attempt %d/%d): %v", attempt+1, r.config.MaxRetries, err)
			time.Sleep(backoffFunc(ctx, attempt+1))
		}
	}

	return nil, &RetryExhaustedError{LastErr: lastErr, TotalRetries: r.config.MaxRetries}
}

func (r *retryChatModel) generateWithProxyCallbacks(ctx context.Context,
	input []*schema.Message, opts ...model.Option) (*schema.Message, error) {

	ctx = callbacks.EnsureRunInfo(ctx, r.GetType(), components.ComponentOfChatModel)
	nCtx := callbacks.OnStart(ctx, &model.CallbackInput{Messages: input})

	out, err := r.inner.Generate(nCtx, input, opts...)
	if err != nil {
		callbacks.OnError(nCtx, err)
		return nil, err
	}

	callbacks.OnEnd(nCtx, &model.CallbackOutput{Message: out})
	return out, nil
}

type streamRetryKey struct{}

type streamRetryInfo struct {
	attempt int // first request is 0, first retry is 1
}

func getStreamRetryInfo(ctx context.Context) (*streamRetryInfo, bool) {
	info, ok := ctx.Value(streamRetryKey{}).(*streamRetryInfo)
	return info, ok
}

// Stream 实现 ChatModel 接口的 Stream 方法，带有重试逻辑。
func (r *retryChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (
	*schema.StreamReader[*schema.Message], error) {

	isRetryAble := r.config.IsRetryAble
	if isRetryAble == nil {
		isRetryAble = defaultIsRetryAble
	}
	backoffFunc := r.config.BackoffFunc
	if backoffFunc == nil {
		backoffFunc = defaultBackoff
	}

	retryInfo := &streamRetryInfo{}
	ctx = context.WithValue(ctx, streamRetryKey{}, retryInfo)

	var lastErr error
	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		retryInfo.attempt = attempt
		var stream *schema.StreamReader[*schema.Message]
		var err error

		if r.innerHandlesCallbacks {
			stream, err = r.inner.Stream(ctx, input, opts...)
		} else {
			stream, err = r.streamWithProxyCallbacks(ctx, input, opts...)
		}

		if err != nil {
			if !isRetryAble(ctx, err) {
				return nil, err
			}
			lastErr = err
			if attempt < r.config.MaxRetries {
				log.Printf("retrying ChatModel.Stream (attempt %d/%d): %v", attempt+1, r.config.MaxRetries, err)
				time.Sleep(backoffFunc(ctx, attempt+1))
			}
			continue
		}

		copies := stream.Copy(2)
		checkCopy := copies[0]
		returnCopy := copies[1]

		streamErr := consumeStreamForError(checkCopy)
		if streamErr == nil {
			return returnCopy, nil
		}

		returnCopy.Close()
		if !isRetryAble(ctx, streamErr) {
			return nil, streamErr
		}

		lastErr = streamErr
		if attempt < r.config.MaxRetries {
			log.Printf("retrying ChatModel.Stream (attempt %d/%d): %v", attempt+1, r.config.MaxRetries, streamErr)
			time.Sleep(backoffFunc(ctx, attempt+1))
		}
	}

	return nil, &RetryExhaustedError{LastErr: lastErr, TotalRetries: r.config.MaxRetries}
}

// streamWithProxyCallbacks 在没有内置回调支持的情况下，使用代理回调执行 Stream。
func (r *retryChatModel) streamWithProxyCallbacks(ctx context.Context,
	input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {

	ctx = callbacks.EnsureRunInfo(ctx, r.GetType(), components.ComponentOfChatModel)
	nCtx := callbacks.OnStart(ctx, &model.CallbackInput{Messages: input})

	stream, err := r.inner.Stream(nCtx, input, opts...)
	if err != nil {
		callbacks.OnError(nCtx, err)
		return nil, err
	}

	out := schema.StreamReaderWithConvert(stream, func(m *schema.Message) (*model.CallbackOutput, error) {
		return &model.CallbackOutput{Message: m}, nil
	})
	_, out = callbacks.OnEndWithStreamOutput(nCtx, out)
	return schema.StreamReaderWithConvert(out, func(o *model.CallbackOutput) (*schema.Message, error) {
		return o.Message, nil
	}), nil
}

// consumeStreamForError 消费流以检查是否存在错误。
func consumeStreamForError(stream *schema.StreamReader[*schema.Message]) error {
	defer stream.Close()
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// GetType 实现 components.Typer 接口。
func (r *retryChatModel) GetType() string {
	if gt, ok := r.inner.(components.Typer); ok {
		return gt.GetType()
	}
	return generic.ParseTypeName(reflect.ValueOf(r.inner))
}

// IsCallbacksEnabled 实现 components.Checker 接口。
func (r *retryChatModel) IsCallbacksEnabled() bool { return true }
