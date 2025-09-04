package handlers

func listResult[T any](list []T) any {
	if list == nil {
		list = make([]T, 0)
	}

	return H{
		"list": list,
	}
}
