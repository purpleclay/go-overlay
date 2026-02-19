package main

import (
	"fmt"
	"runtime"
)

func main() {
	fmt.Printf("hello from Go %s\n", runtime.Version())
}
