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
	"regexp"
	"runtime"
	"strings"
)

var (
	regOfAnonymousFunc = regexp.MustCompile(`^func[0-9]+`)
	regOfNumber        = regexp.MustCompile(`^\d+$`)
)

// ParseTypeName returns the name of the type of the given value.
// It takes a reflect.Value as input and processes it to determine the underlying type. If the type is a pointer, it dereferences it to get the actual type. (the optimization of this function)
// eg: ParseTypeName(reflect.ValueOf(&&myStruct{})) returns "myStruct" (not "**myStruct")
//
// If the type is a function, it retrieves the function's name, handling both named and anonymous functions.
// examples of function paths: [package_path].[receiver_type].[func_name]
// named function: xxx/utils.ParseTypeName
// method: xxx/utils.(*MyStruct).Method
// anonymous function: xxx/utils.TestParseTypeName.func6.1
// ParseTypeName 返回值的“类型名称”（指针会被解引用）
// - 指针：逐层解引用后返回底层类型名
// - 函数：通过 PC 查找函数名，过滤匿名函数/纯数字占位
// - 示例：ParseTypeName(reflect.ValueOf(&&MyStruct{})) => "MyStruct"
func ParseTypeName(val reflect.Value) string {
	typ := val.Type()

	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}

	if typ.Kind() == reflect.Func {
		funcName := runtime.FuncForPC(val.Pointer()).Name()
		idx := strings.LastIndex(funcName, ".")
		if idx < 0 {
			if funcName != "" {
				return funcName
			}
			return ""
		}

		name := funcName[idx+1:]

		if regOfAnonymousFunc.MatchString(name) {
			return ""
		}

		if regOfNumber.MatchString(name) {
			return ""
		}

		return name
	}

	return typ.Name()
}
