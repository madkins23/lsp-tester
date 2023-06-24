package data

// TODO: Connect this to command line flag?
const maxDisplayLen = 32

type AnyMap map[string]any

func (am AnyMap) HasField(name string) bool {
	_, found := am[name]
	return found
}

func (am AnyMap) GetField(name string) (any, bool) {
	if something, found := am[name]; found && something != nil {
		return something, true
	}
	return nil, false
}

func (am AnyMap) GetIntField(name string) (int, bool) {
	if field, found := am.GetField(name); found {
		if result, ok := field.(int); ok {
			return result, true
		}
	}
	return 0, false
}

func (am AnyMap) GetStringField(name string) (string, bool) {
	if field, found := am.GetField(name); found {
		if result, ok := field.(string); ok && result != "" {
			return result, true
		}
	}
	return "", false
}
