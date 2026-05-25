package entity

// Error is the canonical domain error. msg is user-safe and contract-stable.
// metric is a low-cardinality label for errors_total (prevents prom
// cardinality bombs). code drives HTTP status via the error hook.
type Error struct {
	msg    string
	metric string
	code   Code
}

func (e *Error) Error() string  { return e.msg }
func (e *Error) Code() Code     { return e.code }
func (e *Error) Metric() string { return e.metric }

// New constructs a sentinel domain error. Each slice declares its own
// sentinels via this constructor (see internal/domain/entity/<slice>/errors.go).
func New(msg, metric string, code Code) *Error {
	return &Error{msg: msg, metric: metric, code: code}
}
