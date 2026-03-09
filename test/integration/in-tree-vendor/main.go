package main

import (
	"fmt"

	"github.com/rs/xid"
)

func main() {
	id := xid.New()
	fmt.Println("in-tree vendor test:", id.String())
}
