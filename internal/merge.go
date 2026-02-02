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

import (
	"fmt"
	"reflect"

	"github.com/cloudwego/eino/internal/generic"
)

var mergeFuncs = map[reflect.Type]any{}

// RegisterValuesMergeFunc 注册指定类型的“值合并”函数
//   - 适用用户：框架内部与高级用户扩展自定义合并策略
//   - 使用方式：在编译期或初始化阶段调用一次，用于后续合并同类型的值切片
//   - 示例：
//     RegisterValuesMergeFunc[string](func(ss []string) (string, error) { return strings.Join(ss, ""), nil })
func RegisterValuesMergeFunc[T any](fn func([]T) (T, error)) {
	mergeFuncs[generic.TypeOf[T]()] = fn
}

// GetMergeFunc 获取指定类型的通用“值合并”适配器
// - 返回的函数签名为 func([]any) (any, error)，用于在运行时将同类型值统一合并为一个值
// - 优先使用注册的合并函数；若为 map 类型，则内置提供 map 合并策略；否则返回 nil
// - 注意：传入的切片元素必须与 typ 一致，否则返回类型不匹配错误
func GetMergeFunc(typ reflect.Type) func([]any) (any, error) {
	if fn, ok := mergeFuncs[typ]; ok {
		return func(vs []any) (any, error) {
			// 将 []any 转为具体类型的切片，做运行时类型校验
			rvs := reflect.MakeSlice(reflect.SliceOf(typ), 0, len(vs))
			for _, v := range vs {
				if t := reflect.TypeOf(v); t != typ {
					return nil, fmt.Errorf(
						"(values merge) field type mismatch. expected: '%v', got: '%v'", typ, t)
				}
				rvs = reflect.Append(rvs, reflect.ValueOf(v))
			}

			// 通过反射调用注册的具体合并函数
			rets := reflect.ValueOf(fn).Call([]reflect.Value{rvs})
			var err error
			if !rets[1].IsNil() {
				err = rets[1].Interface().(error)
			}
			return rets[0].Interface(), err
		}
	}

	if typ.Kind() == reflect.Map {
		return func(vs []any) (any, error) {
			return mergeMap(typ, vs)
		}
	}

	return nil
}

// mergeMap 合并若干同类型的 map
// - 规则：键不可重复，一旦发现重复键直接返回错误
// - 使用场景：扇入场景下的字典合并，将不同来源的键值统一并入一个 map
func mergeMap(typ reflect.Type, vs []any) (any, error) {
	merged := reflect.MakeMap(typ)
	for _, v := range vs {
		if t := reflect.TypeOf(v); t != typ {
			return nil, fmt.Errorf(
				"(values merge map) field type mismatch. expected: '%v', got: '%v'", typ, t)
		}

		// 迭代原 map，将键值写入目标 merged
		iter := reflect.ValueOf(v).MapRange()
		for iter.Next() {
			key, val := iter.Key(), iter.Value()
			if merged.MapIndex(key).IsValid() {
				return nil, fmt.Errorf("(values merge map) duplicated key ('%v') found", key.Interface())
			}
			merged.SetMapIndex(key, val)
		}
	}

	return merged.Interface(), nil
}
