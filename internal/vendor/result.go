package vendor

// Status represents the outcome of processing a single go.mod or go.work.
type Status string

const (
	StatusOK        Status = "ok"
	StatusGenerated Status = "generated"
	StatusDrift     Status = "drift"
	StatusMissing   Status = "missing"
	StatusSkipped   Status = "skipped"
	StatusError     Status = "error"
)

func (s Status) IsSuccess() bool {
	return s == StatusOK || s == StatusGenerated || s == StatusSkipped
}

func (s Status) IsFailure() bool {
	return s == StatusDrift || s == StatusMissing || s == StatusError
}

// Result captures the outcome of processing a single file.
type Result struct {
	Path    string
	Status  Status
	Message string
}
