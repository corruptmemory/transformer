package util

type MultiError struct {
	Errors []error
}

func (m *MultiError) Error() string {
	var s string
	for _, e := range m.Errors {
		s += e.Error() + "\n"
	}
	return s
}

func NewMultiError() *MultiError {
	return &MultiError{}
}

func (m *MultiError) Add(err error) {
	m.Errors = append(m.Errors, err)
}

func (m *MultiError) HasErrors() bool {
	return len(m.Errors) > 0
}
