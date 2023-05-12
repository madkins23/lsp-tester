package main

type genericData map[string]any

func makeGeneric(data map[string]interface{}) *genericData {
	result := &genericData{}
	for key, value := range data {
		(*result)[key] = value.(any)
	}
	return result
}

func (gd genericData) hasField(name string) bool {
	_, found := gd[name]
	return found
}

func (gd genericData) getField(name string) (any, bool) {
	if something, found := gd[name]; found && something != nil {
		return something, true
	}
	return nil, false
}

func (gd genericData) getIntField(name string) (int, bool) {
	if field, found := gd.getField(name); found {
		if result, ok := field.(int); ok {
			return result, true
		}
	}
	return 0, false
}

func (gd genericData) getStringField(name string) (string, bool) {
	if field, found := gd.getField(name); found {
		if result, ok := field.(string); ok && result != "" {
			return result, true
		}
	}
	return "", false
}
