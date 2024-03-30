package slicekit

import (
	"cmp"
	"slices"
)

func Range(start int, end int) []int {
	if start >= end {
		return nil
	}

	size := end - start
	result := make([]int, size)
	for i := 0; i < size; i++ {
		result[i] = start + i
	}
	return result
}

func MapWithIndex[T any, R any](items []T, mapper func(int, T) R) []R {
	if len(items) == 0 {
		return nil
	}

	result := make([]R, len(items))
	for i, item := range items {
		result[i] = mapper(i, item)
	}
	return result
}

func Map[T any, R any](items []T, mapper func(T) R) []R {
	return MapWithIndex(items, func(i int, t T) R {
		return mapper(t)
	})
}

func SortFuncWithIndex[T any](items []T, cmp func(i1, i2 int, e1, e2 T) int) []T {
	if len(items) <= 1 {
		return slices.Clone(items)
	}

	indexes := Range(0, len(items))
	slices.SortFunc(indexes, func(i1, i2 int) int {
		return cmp(i1, i2, items[i1], items[i2])
	})

	return Map(indexes, func(i int) T { return items[i] })
}

func SortByWithIndex[T any, V cmp.Ordered](items []T, byFunc func(int, T) V) []T {
	by := MapWithIndex(items, byFunc)
	return SortFuncWithIndex(items, func(i1, i2 int, _, _ T) int {
		return cmp.Compare(by[i1], by[i2])
	})
}

func KeyBy[T any, K comparable](items []T, f func(T) K) map[K]T {
	r := make(map[K]T, len(items))
	for _, item := range items {
		r[f(item)] = item
	}
	return r
}
