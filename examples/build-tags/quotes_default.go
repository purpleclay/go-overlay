//go:build !meaningful && !procrastination

package main

import "github.com/fatih/color"

var printer = color.New(color.FgYellow)

var quotes = []string{
	"You compiled me without a tag. I have no strong opinions.",
	"Neither motivated nor procrastinating. Just... here.",
	"A quote without a tag is like a commit without a message.",
	"Fortune favours the build flag.",
	"Try again with -tags meaningful or -tags procrastination.",
}
