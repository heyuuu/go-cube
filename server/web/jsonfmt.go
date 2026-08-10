package web

import (
	"encoding/json"
	"io"
	"reflect"

	"github.com/danielgtaylor/huma/v2"
)

// nilSliceJSONFormat 替换 huma 默认 JSON 格式：把响应中的 nil 切片序列化为 []，
// nil map 序列化为 {}（标准库默认输出 null）。
//
// 这是纯展示层关注点——前端 TS 类型声明为 string[] / Record，拿到 null 会触发
// 运行时崩溃。内部业务代码无需关心 nil/empty 区别（Go 里 nil slice 本就可安全
// range/append），这层只负责给前端一个稳定的 JSON 形态。
//
// 未来 Go 1.27 落地 encoding/json/v2 后（v2 默认 nil→[]），整个文件可删除、
// server.go 还原为 huma.DefaultConfig 即可。
var nilSliceJSONFormat = huma.Format{
	Marshal: func(w io.Writer, v any) error {
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false) // 与 huma DefaultJSONFormat 行为对齐
		return enc.Encode(replaceNilSlices(v))
	},
	Unmarshal: json.Unmarshal, // 入站不改：前端发 null 仍正常解析为 nil
}

// replaceNilSlices 递归把 v 中所有 nil 切片/map 替换为对应类型的空集合。
// 返回新构造的值，不修改原对象（值类型天然隔离；struct 经由深拷贝副本）。
func replaceNilSlices(v any) any {
	if v == nil {
		return nil
	}
	return replaceNilSlicesValue(reflect.ValueOf(v)).Interface()
}

// replaceNilSlicesValue 是 reflect 层的递归实现，返回替换后的 reflect.Value。
func replaceNilSlicesValue(rv reflect.Value) reflect.Value {
	// 解包指针/接口，拿到底层值；nil 指针/接口原样返回（输出 null 是合理的）。
	for rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return rv
		}
		// 指针/接口本身保留包装，递归处理其元素后重新包装回去。
		elem := replaceNilSlicesValue(rv.Elem())
		// 指针指向的若是新构造的副本，需更新指针；
		// 但直接 SetElem 受 CanAddr 限制——改为返回新指针更稳妥。
		if rv.Kind() == reflect.Pointer {
			out := reflect.New(elem.Type())
			out.Elem().Set(elem)
			return out
		}
		return elem
	}

	switch rv.Kind() {
	case reflect.Slice:
		if rv.IsNil() {
			return reflect.MakeSlice(rv.Type(), 0, 0) // nil → 空切片
		}
		// 非 nil 切片：逐元素递归（元素可能是 struct，内含 nil 切片字段）。
		out := reflect.MakeSlice(rv.Type(), rv.Len(), rv.Cap())
		for i := 0; i < rv.Len(); i++ {
			out.Index(i).Set(replaceNilSlicesValue(rv.Index(i)))
		}
		return out

	case reflect.Map:
		if rv.IsNil() {
			return reflect.MakeMap(rv.Type()) // nil → 空 map
		}
		out := reflect.MakeMapWithSize(rv.Type(), rv.Len())
		for iter := rv.MapRange(); iter.Next(); {
			out.SetMapIndex(iter.Key(), replaceNilSlicesValue(iter.Value()))
		}
		return out

	case reflect.Struct:
		// 构造副本，逐字段递归替换（跳过 unexported 字段，避免 panic）。
		out := reflect.New(rv.Type()).Elem()
		for i := 0; i < rv.NumField(); i++ {
			if !rv.Type().Field(i).IsExported() {
				continue
			}
			out.Field(i).Set(replaceNilSlicesValue(rv.Field(i)))
		}
		return out
	}

	return rv
}
