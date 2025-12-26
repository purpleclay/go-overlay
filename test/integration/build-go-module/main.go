package main

import (
	"fmt"
	"runtime"
)

func main() {
	fmt.Printf("go-overlay integration test (built with %s)\n", runtime.Version())
}
