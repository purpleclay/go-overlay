package progress

// NopReporter discards every event. Selected when progress reporting is
// disabled (--no-progress) or stderr isn't a terminal worth writing an
// interactive view to.
type NopReporter struct{}

func (NopReporter) Report(Event) {}
