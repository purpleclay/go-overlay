//go:build js && wasm

package main

import (
	"strings"
	"syscall/js"

	"github.com/rivo/uniseg"
)

func analyse(text string) (words, lines, chars int) {
	if text == "" {
		return 0, 0, 0
	}
	words = len(strings.Fields(text))
	lines = strings.Count(text, "\n") + 1
	chars = uniseg.GraphemeClusterCount(text)
	return
}

func main() {
	js.Global().Set("analyseText", js.FuncOf(func(this js.Value, args []js.Value) any {
		text := args[0].String()
		words, lines, chars := analyse(text)

		result := js.Global().Get("Object").New()
		result.Set("words", words)
		result.Set("lines", lines)
		result.Set("chars", chars)
		return result
	}))

	// Keep the Go runtime alive — WASM programs must not exit or the
	// exported functions become unavailable in the calling JS environment.
	select {}
}
