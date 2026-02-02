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

package schema

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/cloudwego/eino/internal/serialization"
)

type testStruct struct{}

// TestGetTypeName 测试获取类型名称的功能。
// 验证不同类型（命名类型、指针、Map、内置类型、标准库类型、本地类型）
// 能否正确生成符合预期的类型名称字符串。
func TestGetTypeName(t *testing.T) {
	type localNamedType struct{}

	testCases := []struct {
		name     string
		input    reflect.Type
		expected string
	}{
		{
			name:     "named type from current package",
			input:    reflect.TypeOf(testStruct{}),
			expected: "github.com/cloudwego/eino/schema.testStruct",
		},
		{
			name:     "pointer to named type from current package",
			input:    reflect.TypeOf(&testStruct{}),
			expected: "*github.com/cloudwego/eino/schema.testStruct",
		},
		{
			name:     "unnamed map type",
			input:    reflect.TypeOf(map[string]int{}),
			expected: "map[string]int",
		},
		{
			name:     "pointer to unnamed map type",
			input:    reflect.TypeOf(new(map[string]int)),
			expected: "*map[string]int",
		},
		{
			name:     "built-in type",
			input:    reflect.TypeOf(0),
			expected: "int",
		},
		{
			name:     "pointer to built-in type",
			input:    reflect.TypeOf(new(int)),
			expected: "*int",
		},
		{
			name:     "named type from standard library",
			input:    reflect.TypeOf(bytes.Buffer{}),
			expected: "bytes.Buffer",
		},
		{
			name:     "pointer to named type from standard library",
			input:    reflect.TypeOf(&bytes.Buffer{}),
			expected: "*bytes.Buffer",
		},
		{
			name:     "local named type",
			input:    reflect.TypeOf(localNamedType{}),
			expected: "github.com/cloudwego/eino/schema.localNamedType",
		},
		{
			name:     "pointer to local named type",
			input:    reflect.TypeOf(&localNamedType{}),
			expected: "*github.com/cloudwego/eino/schema.localNamedType",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := getTypeName(tc.input)
			if actual != tc.expected {
				t.Errorf("getTypeName() got %q, want %q", actual, tc.expected)
			}
		})
	}
}

// TestRegister 测试类型注册和序列化功能。
// 验证：
// 1. 注册不同类型的结构体（包括嵌套结构体、Slice、Map）。
// 2. 使用 InternalSerializer 进行 Marshal 和 Unmarshal，验证数据一致性。
// 3. 使用 gob 进行 Encode 和 Decode，验证兼容性。
// 4. 验证重复注册不会引发 Panic。
func TestRegister(t *testing.T) {
	type testStruct1 struct {
		A any
		B any
		C any
		D any
		E any
		F any
	}

	type testStruct2 struct{}

	Register[*testStruct1]()
	Register[*testStruct2]()
	Register[[]Message]()
	Register[[]*testStruct2]()
	Register[[]testStruct2]()

	t1 := testStruct1{A: []*Message{{}}, B: []Message{{}}, C: []*testStruct2{{}}, D: []testStruct2{{}},
		E: &testStruct1{}, F: []int{1}}

	in := &serialization.InternalSerializer{}
	mar, err := in.Marshal(t1)
	if err != nil {
		panic(err)
	}
	var t2 testStruct1
	err = in.Unmarshal(mar, &t2)
	if err != nil {
		panic(err)
	}
	assert.Equal(t, t1, t2)

	buf := new(bytes.Buffer)
	err = gob.NewEncoder(buf).Encode(t1)
	if err != nil {
		panic(err)
	}
	err = gob.NewDecoder(buf).Decode(&t2)
	if err != nil {
		panic(err)
	}
	assert.Equal(t, t1, t2)

	f := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic: %v", r)
			}
		}()

		Register[[]int]()
		Register[map[string]any]()
		Register[[]*testStruct1]()
		Register[[]testStruct1]()

		return nil
	}

	err = f()
	assert.NoError(t, err)
}

// TestRegisterStructWithUUIDField 重现并验证 issue #607。
// 验证包含 uuid.UUID 字段的结构体在注册和序列化/反序列化时是否正常工作。
// 之前版本中，uuid.UUID 字段可能导致反序列化 Panic。
func TestRegisterStructWithUUIDField(t *testing.T) {
	type Item struct {
		ID uuid.UUID
	}

	RegisterName[Item]("test_item")

	original := Item{
		ID: uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
	}

	s := &serialization.InternalSerializer{}

	data, err := s.Marshal(original)
	assert.NoError(t, err)

	var result Item
	err = s.Unmarshal(data, &result)
	assert.NoError(t, err)

	assert.Equal(t, original.ID, result.ID)
}
