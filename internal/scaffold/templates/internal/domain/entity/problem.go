package entity

// Problem is the canonical domain error, shaped after RFC 7807. msg is
// user-safe and contract-stable (becomes "detail" in the response body);
// metric is a low-cardinality label for errors_total to keep prometheus
// cardinality bounded; status is the HTTP status the error maps to —
// net/http constants are the source of truth, no parallel enum.
type Problem struct {
	msg    string
	metric string
	status int
}

func (p *Problem) Error() string  { return p.msg }
func (p *Problem) Status() int    { return p.status }
func (p *Problem) Metric() string { return p.metric }

// NewProblem constructs a domain error. Each slice declares its own
// sentinels via this constructor (see internal/domain/entity/<slice>/errors.go).
func NewProblem(msg, metric string, status int) *Problem {
	return &Problem{msg: msg, metric: metric, status: status}
}
