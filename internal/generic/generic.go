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

package generic

import (
	"reflect"
)

// NewInstance create an instance of the given type T.
// the main purpose of this function is to create an instance of a type, can handle the type of T is a pointer or not.
// eg. NewInstance[int] returns 0.
// eg. NewInstance[*int] returns *0 (will be ptr of 0, not nil!).
// NewInstance 根据类型 T 创建一个“已初始化”的实例
// - 指针类型：逐层分配直到最底层，返回非 nil 的多层指针
// - Map/Slice/Array：返回空实例
// - 标量类型：返回零值
// - 使用示例：NewInstance[*int]() 返回 **int 的非 nil 链
func NewInstance[T any]() T {
	typ := TypeOf[T]()

	switch typ.Kind() {
	case reflect.Map:
		return reflect.MakeMap(typ).Interface().(T)
	case reflect.Slice, reflect.Array:
		return reflect.MakeSlice(typ, 0, 0).Interface().(T)
	case reflect.Ptr:
		typ = typ.Elem()
		origin := reflect.New(typ)
		inst := origin

		for typ.Kind() == reflect.Ptr {
			typ = typ.Elem()
			inst = inst.Elem()
			inst.Set(reflect.New(typ))
		}

		return origin.Interface().(T)
	default:
		var t T
		return t
	}
}

// TypeOf returns the type of T.
// eg. TypeOf[int] returns reflect.TypeOf(int).
// eg. TypeOf[*int] returns reflect.TypeOf(*int).
// TypeOf 获取类型形参 T 对应的反射类型
func TypeOf[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}

// PtrOf returns a pointer of T.
// useful when you want to get a pointer of a value, in some config, for example.
// eg. PtrOf[int] returns *int.
// eg. PtrOf[*int] returns **int.
// PtrOf 返回值 v 的指针，常用于配置项、回调等需要指针的场景
func PtrOf[T any](v T) *T {
	return &v
}

type Pair[F, S any] struct {
	First  F
	Second S
}

// Reverse returns a new slice with elements in reversed order.
// Reverse 返回反转后的新切片
func Reverse[S ~[]E, E any](s S) S {
	d := make(S, len(s))
	for i := 0; i < len(s); i++ {
		d[i] = s[len(s)-i-1]
	}

	return d
}

// CopyMap copies a map to a new map.
// CopyMap 复制 map 的键值到一个新 map（浅拷贝）
func CopyMap[K comparable, V any](src map[K]V) map[K]V {
	dst := make(map[K]V, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
