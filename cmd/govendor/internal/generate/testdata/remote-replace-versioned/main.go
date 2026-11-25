package main

import (
	"fmt"

	"github.com/pkg/errors"
)

func main() {
	fmt.Println(errors.New("hello from go-overlay"))
}
