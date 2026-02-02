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

package internal

import "sync"

// UnboundedChan 表示一个“无限缓冲”的通道实现
// - 适用用户：框架内部并发通信，或用户在需要非阻塞写入、阻塞读取的队列场景
// - 使用方式：NewUnboundedChan[T]() 创建实例，Send 写入，Receive 阻塞读取，Close 关闭
// - 注意：关闭后写入将触发 panic；读取在空且未关闭时阻塞，在空且已关闭时返回 (零值,false)
type UnboundedChan[T any] struct {
	buffer   []T        // Internal buffer to store data
	mutex    sync.Mutex // Mutex to protect buffer access
	notEmpty *sync.Cond // Condition variable to wait for data
	closed   bool       // Indicates if the channel has been closed
}

// NewUnboundedChan 初始化并返回一个 UnboundedChan
// - 初始化条件变量，并将其与互斥锁绑定
func NewUnboundedChan[T any]() *UnboundedChan[T] {
	ch := &UnboundedChan[T]{}
	ch.notEmpty = sync.NewCond(&ch.mutex)
	return ch
}

// Send 将一个元素放入通道
// - 写入后通过 Signal 唤醒一个等待接收的协程
// - 若通道已关闭则触发 panic
func (ch *UnboundedChan[T]) Send(value T) {
	ch.mutex.Lock()
	defer ch.mutex.Unlock()

	if ch.closed {
		panic("send on closed channel")
	}

	ch.buffer = append(ch.buffer, value)
	ch.notEmpty.Signal() // Wake up one goroutine waiting to receive
}

// Receive 从通道取出一个元素（在为空时阻塞）
// - 当缓冲区为空且未关闭时，使用条件变量等待
// - 当缓冲区为空且已关闭时，返回 (零值,false)
// - 正常读取路径返回 (val,true)
func (ch *UnboundedChan[T]) Receive() (T, bool) {
	ch.mutex.Lock()
	defer ch.mutex.Unlock()

	for len(ch.buffer) == 0 && !ch.closed {
		ch.notEmpty.Wait() // Wait until data is available
	}

	if len(ch.buffer) == 0 {
		// Channel is closed and empty
		var zero T
		return zero, false
	}

	val := ch.buffer[0]
	ch.buffer = ch.buffer[1:]
	return val, true
}

// Close 将通道标记为已关闭
// - 广播唤醒所有等待接收的协程，避免永久阻塞
func (ch *UnboundedChan[T]) Close() {
	ch.mutex.Lock()
	defer ch.mutex.Unlock()

	if !ch.closed {
		ch.closed = true
		ch.notEmpty.Broadcast() // Wake up all waiting goroutines
	}
}
