package config

var isDebug = false

func SetDebug(debug bool) {
	isDebug = debug
}

func IsDebug() bool {
	return isDebug
}
