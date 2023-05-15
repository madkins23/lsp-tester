package data

type AnyMap map[string]any

func (gd AnyMap) HasField(name string) bool {
	_, found := gd[name]
	return found
}

func (gd AnyMap) GetField(name string) (any, bool) {
	if something, found := gd[name]; found && something != nil {
		return something, true
	}
	return nil, false
}

func (gd AnyMap) GetIntField(name string) (int, bool) {
	if field, found := gd.GetField(name); found {
		if result, ok := field.(int); ok {
			return result, true
		}
	}
	return 0, false
}

func (gd AnyMap) GetStringField(name string) (string, bool) {
	if field, found := gd.GetField(name); found {
		if result, ok := field.(string); ok && result != "" {
			return result, true
		}
	}
	return "", false
}
