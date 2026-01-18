package main

import (
	"fmt"

	"github.com/sourcegraph/conc/pool"

	_ "github.com/btcsuite/btcd/btcutil"
	_ "github.com/btcsuite/btcd/chaincfg/chainhash"
)

func main() {
	p := pool.New()
	p.Go(func() {
		fmt.Println("hello from pool")
	})
	p.Wait()
}
