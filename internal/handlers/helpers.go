package handlers

import (
	"encoding/json"
)

func map2struct[T any](data any) (obj T, err error) {
	var bin []byte

	bin, err = json.Marshal(data)
	if err != nil {
		return
	}

	err = json.Unmarshal(bin, &obj)
	return
}
func parseParam[T any](data any) (obj T, err error) {
	// 数据转结构体
	s, err := map2struct[T](data)
	if err != nil {
		return s, err
	}

	// 校验参数
	err = validateStruct(s)
	return s, err
}

func listResult[T any](list []T) any {
	if list == nil {
		list = make([]T, 0)
	}

	return H{
		"list": list,
	}
}

func itemResult[T any, R any](item *T, converter func(*T) R) (any, error) {
	if item == nil {
		return nil, nil
	}
	return converter(item), nil
}
