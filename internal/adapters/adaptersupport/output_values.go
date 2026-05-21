package adaptersupport

import (
	"encoding/json"
	"fmt"
	"strconv"
)

func StringifyOutputValue(value interface{}) (string, error) {
	switch typed := value.(type) {
	case string:
		return typed, nil
	case bool:
		return strconv.FormatBool(typed), nil
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), nil
	case nil:
		return "", nil
	case []interface{}, map[string]interface{}:
		data, err := json.Marshal(typed)
		if err != nil {
			return "", err
		}
		return string(data), nil
	default:
		return "", fmt.Errorf("unsupported output value type %T", value)
	}
}
