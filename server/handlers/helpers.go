package handlers

import (
	"cube/web"
)

func listResult[T any](list []T) web.ListResult[T] {
	return web.ListResult[T]{List: list}
}
