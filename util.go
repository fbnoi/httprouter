package httprouter

import (
	"fmt"
	"path"
	"strconv"
)

func cleanPath(p string) string {
	if p == "" {
		return "/"
	}

	return path.Clean(p)
}

func interfaceToString(value interface{}) string {
	var key string
	if value == nil {
		return key
	}
	switch conv := value.(type) {
	case float64:
		key = strconv.FormatFloat(conv, 'f', -1, 64)
	case float32:
		key = strconv.FormatFloat(float64(conv), 'f', -1, 64)
	case int:
		key = strconv.Itoa(conv)
	case uint:
		key = strconv.Itoa(int(conv))
	case int8:
		key = strconv.Itoa(int(conv))
	case uint8:
		key = strconv.Itoa(int(conv))
	case int16:
		key = strconv.Itoa(int(conv))
	case uint16:
		key = strconv.Itoa(int(conv))
	case int32:
		key = strconv.Itoa(int(conv))
	case uint32:
		key = strconv.Itoa(int(conv))
	case int64:
		key = strconv.FormatInt(conv, 10)
	case uint64:
		key = strconv.FormatUint(conv, 10)
	case string:
		key = value.(string)
	case []byte:
		key = string(value.([]byte))
	case bool:
		key = strconv.FormatBool(conv)
	default:
		panic(fmt.Sprintf("%v convert to string failed", value))
	}
	return key
}
