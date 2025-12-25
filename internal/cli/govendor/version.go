package govendor

import (
	"fmt"
	"runtime"
	"strings"
)

type BuildDetails struct {
	Version   string
	Commit    string
	BuildDate string
	Go        string
	GoArch    string
	GoOS      string
}

func NewBuildDetails(version, commit, buildDate string) BuildDetails {
	return BuildDetails{
		Version:   version,
		Commit:    commit,
		BuildDate: buildDate,
		Go:        runtime.Version(),
		GoArch:    runtime.GOARCH,
		GoOS:      runtime.GOOS,
	}
}

func (b BuildDetails) String() string {
	var buf strings.Builder
	buf.WriteString(fmt.Sprintf("version: %s\n", b.Version))
	buf.WriteString(fmt.Sprintf("go:      %s (%s/%s)\n", strings.TrimPrefix(b.Go, "go"), b.GoOS, b.GoArch))
	buf.WriteString(fmt.Sprintf("commit:  %s\n", b.Commit))
	buf.WriteString(fmt.Sprintf("built:   %s\n", b.BuildDate))
	return buf.String()
}
