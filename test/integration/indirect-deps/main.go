package main

import (
	"fmt"

	"github.com/sourcegraph/conc/pool"
)

func main() {
	p := pool.New()
	p.Go(func() {
		fmt.Println("hello from pool")
	})
	p.Wait()
}
