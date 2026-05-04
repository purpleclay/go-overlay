package main

import (
	"fmt"

	"github.com/go-ini/ini"
)

func main() {
	cfg := ini.Empty()
	cfg.Section("").Key("greeting").SetValue("hello from go-overlay")
	fmt.Println(cfg.Section("").Key("greeting").String())
}
