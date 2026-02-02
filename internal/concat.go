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
	"strings"
	"time"

	"github.com/cloudwego/eino/internal/generic"
)

var (
	concatFuncs = map[reflect.Type]any{
		generic.TypeOf[string]():        concatStrings,
		generic.TypeOf[int8]():          useLast[int8],
		generic.TypeOf[int16]():         useLast[int16],
		generic.TypeOf[int32]():         useLast[int32],
		generic.TypeOf[int64]():         useLast[int64],
		generic.TypeOf[int]():           useLast[int],
		generic.TypeOf[uint8]():         useLast[uint8],
		generic.TypeOf[uint16]():        useLast[uint16],
		generic.TypeOf[uint32]():        useLast[uint32],
		generic.TypeOf[uint64]():        useLast[uint64],
		generic.TypeOf[uint]():          useLast[uint],
		generic.TypeOf[bool]():          useLast[bool],
		generic.TypeOf[float32]():       useLast[float32],
		generic.TypeOf[float64]():       useLast[float64],
		generic.TypeOf[time.Time]():     useLast[time.Time],
		generic.TypeOf[time.Duration](): useLast[time.Duration],
	}
)

// useLast 将切片的最后一个非空值作为合并结果返回
// - 适用用户：框架内部默认策略，也可供高级用户理解/扩展
// - 使用说明：用于数值、时间等标量类型的流块合并，直接取最新值
func useLast[T any](s []T) (T, error) {
	return s[len(s)-1], nil
}

// concatStrings 高效拼接字符串切片
// - 逻辑：预先计算总长度，并使用 strings.Builder 进行一次性分配以提升性能
// - 注意：若中途 WriteString 发生错误，直接返回该错误
func concatStrings(ss []string) (string, error) {
	var n int
	for _, s := range ss {
		n += len(s)
	}

	var b strings.Builder
	b.Grow(n)
	for _, s := range ss {
		_, err := b.WriteString(s)
		if err != nil {
			return "", err
		}
	}

	return b.String(), nil
}

// RegisterStreamChunkConcatFunc 注册指定类型的流块拼接函数
// - 适用用户：框架内部与高级用户，为自定义类型提供合并策略
// - 示例：RegisterStreamChunkConcatFunc[[]byte](func(bs [][]byte) ([]byte, error) { ... })
func RegisterStreamChunkConcatFunc[T any](fn func([]T) (T, error)) {
	concatFuncs[generic.TypeOf[T]()] = fn
}

// GetConcatFunc 获取指定元素类型的拼接函数
// - 返回函数签名：func(reflect.Value) (reflect.Value, error)
// - 若已注册则返回对应函数，否则返回 nil
func GetConcatFunc(typ reflect.Type) func(reflect.Value) (reflect.Value, error) {
	if fn, ok := concatFuncs[typ]; ok {
		return func(a reflect.Value) (reflect.Value, error) {
			// 通过反射调用已注册的合并函数
			rvs := reflect.ValueOf(fn).Call([]reflect.Value{a})
			var err error
			if !rvs[1].IsNil() {
				err = rvs[1].Interface().(error)
			}
			return rvs[0], err
		}
	}

	return nil
}

// ConcatItems the caller should ensure len(items) > 1
// ConcatItems 合并同类型切片中的元素为一个值
// - 前置条件：调用方需保证 len(items) > 1
// - 处理策略：
//   1) 若元素类型为 map：递归合并 map（键不可重复）
//   2) 否则使用已注册的类型拼接函数；未注册时按“单非空值”策略处理
func ConcatItems[T any](items []T) (T, error) {
	typ := generic.TypeOf[T]()
	v := reflect.ValueOf(items)

	var cv reflect.Value
	var err error

	// handle map kind
	if typ.Kind() == reflect.Map {
		cv, err = concatMaps(v)
	} else {
		cv, err = concatSliceValue(v)
	}

	if err != nil {
		var t T
		return t, err
	}

	return cv.Interface().(T), nil
}

// concatMaps 合并若干同类型 map 的所有键值
// - 对每个键聚合其出现的所有值，随后对每个键对应的值切片进行递归合并
// - 注意：当切片仅包含一个元素且该元素为 nil 时，使用类型零值写入（避免 SetMapIndex 删除键）
func concatMaps(ms reflect.Value) (reflect.Value, error) {
	typ := ms.Type().Elem()

	rms := reflect.MakeMap(reflect.MapOf(typ.Key(), generic.TypeOf[[]any]()))
	ret := reflect.MakeMap(typ)

	n := ms.Len()
	for i := 0; i < n; i++ {
		m := ms.Index(i)

		for _, key := range m.MapKeys() {
			vals := rms.MapIndex(key)
			if !vals.IsValid() {
				var s []any
				vals = reflect.ValueOf(s)
			}

			val := m.MapIndex(key)
			vals = reflect.Append(vals, val)
			rms.SetMapIndex(key, vals)
		}
	}

	for _, key := range rms.MapKeys() {
		vals := rms.MapIndex(key)

		anyVals := vals.Interface().([]any)
		if len(anyVals) == 1 {
			ele := anyVals[0]
			if ele == nil { // we cannot SetMapIndex with nil because it will delete the key
				ret.SetMapIndex(key, reflect.Zero(typ.Elem()))
				continue
			}

			ret.SetMapIndex(key, reflect.ValueOf(ele))
			continue
		}

		v, err := toSliceValue(anyVals)
		if err != nil {
			return reflect.Value{}, err
		}

		var cv reflect.Value

		if v.Type().Elem().Kind() == reflect.Map {
			cv, err = concatMaps(v)
		} else {
			cv, err = concatSliceValue(v)
		}

		if err != nil {
			return reflect.Value{}, err
		}

		ret.SetMapIndex(key, cv)
	}

	return ret, nil
}

// concatSliceValue 合并非 map 类型的切片值
// - 优先使用已注册的类型拼接函数
// - 未注册时：若切片全空返回该类型零值；若恰有一个非空元素则返回该元素；否则报错
func concatSliceValue(val reflect.Value) (reflect.Value, error) {
	elmType := val.Type().Elem()

	if val.Len() == 1 {
		return val.Index(0), nil
	}

	f := GetConcatFunc(elmType)
	if f != nil {
		return f(val)
	}

	// if all elements in the slice are empty, return an empty value
	// if there is exactly one non-empty element in the slice, return that non-empty element
	// otherwise, throw an error.
	var filtered reflect.Value
	for i := 0; i < val.Len(); i++ {
		oneVal := val.Index(i)
		if !oneVal.IsZero() {
			if filtered.IsValid() {
				return reflect.Value{}, fmt.Errorf("cannot concat multiple non-zero value of type %s", elmType)
			}

			filtered = oneVal
		}
	}
	if !filtered.IsValid() {
		filtered = reflect.New(elmType).Elem()
	}

	return filtered, nil
}

// toSliceValue 将 []any 转换为具体类型的切片 reflect.Value
// - 保证所有元素类型一致，否则返回错误
func toSliceValue(vs []any) (reflect.Value, error) {
	typ := reflect.TypeOf(vs[0])

	ret := reflect.MakeSlice(reflect.SliceOf(typ), len(vs), len(vs))
	ret.Index(0).Set(reflect.ValueOf(vs[0]))

	for i := 1; i < len(vs); i++ {
		v := vs[i]
		vt := reflect.TypeOf(v)
		if typ != vt {
			return reflect.Value{}, fmt.Errorf("unexpected slice element type. Got %v, expected %v", typ, vt)
		}

		ret.Index(i).Set(reflect.ValueOf(v))
	}

	return ret, nil
}
