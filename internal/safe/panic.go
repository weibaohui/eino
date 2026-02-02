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

package safe

import (
	"fmt"
)

// panicErr 恐慌错误包装结构
// - info: panic 的原始值
// - stack: 堆栈信息
type panicErr struct {
	info  any
	stack []byte
}

func (p *panicErr) Error() string {
	return fmt.Sprintf("panic error: %v, \nstack: %s", p.info, string(p.stack))
}

// NewPanicErr creates a new panic error.
// panicErr is a wrapper of panic info and stack trace.
// it implements the error interface, can print error message of info and stack trace.
//
// NewPanicErr 创建一个新的恐慌错误
// - info: panic 的原始值
// - stack: 堆栈信息
func NewPanicErr(info any, stack []byte) error {
	return &panicErr{
		info:  info,
		stack: stack,
	}
}
