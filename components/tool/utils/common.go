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

package utils

import (
	"github.com/bytedance/sonic"
)

// marshalString converts the response to a string.
// If the response is already a string, it returns it directly.
// Otherwise, it marshals the response to a JSON string.
// marshalString 将响应转换为字符串。
// 如果响应已经是字符串，则直接返回。
// 否则，将其编组为 JSON 字符串。
func marshalString(resp any) (string, error) {
	if rs, ok := resp.(string); ok {
		return rs, nil
	}
	return sonic.MarshalString(resp)
}
