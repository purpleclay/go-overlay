package main

import (
	"fmt"
	"runtime"

	"github.com/fatih/color"
)

var verdicts = map[string]string{
	"darwin":  "you made the right choice.",
	"freebsd": "a little niche, don't you think?",
	"windows": "I bet your company was trying to save money.",
	"linux":   "bold choice for a desktop, solid choice for everything else.",
}

var verdictColors = map[string]*color.Color{
	"darwin":  color.New(color.FgGreen, color.Bold),
	"freebsd": color.New(color.FgYellow, color.Bold),
	"windows": color.New(color.FgRed, color.Bold),
	"linux":   color.New(color.FgCyan, color.Bold),
}

func main() {
	bold := color.New(color.Bold)

	fmt.Printf("OS:   ")
	bold.Printf("%s\n", runtime.GOOS)
	fmt.Printf("Arch: ")
	bold.Printf("%s\n", runtime.GOARCH)
	fmt.Println()

	verdict, ok := verdicts[runtime.GOOS]
	if !ok {
		verdict = fmt.Sprintf("no strong opinions about %s.", runtime.GOOS)
	}

	clr, ok := verdictColors[runtime.GOOS]
	if !ok {
		clr = color.New(color.Reset)
	}

	fmt.Printf("Verdict: ")
	clr.Println(verdict)
}
