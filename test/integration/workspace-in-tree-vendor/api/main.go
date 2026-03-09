package main

import (
	"fmt"

	"example.com/shared"
	"github.com/rs/xid"
)

func main() {
	msg := shared.Hello()
	id := xid.New()
	fmt.Println(msg, id.String())
}
