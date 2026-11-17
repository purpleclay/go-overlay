package main

import (
	"fmt"

	"example.com/nested"
	"example.com/nested/api"
	"example.com/nested/api/legacy"
	"example.com/nested/apiclient"
	ini "github.com/go-ini/ini"
	"github.com/go-ini/ini/testdata"
)

func main() {
	fmt.Println(ini.Empty().Section("").Name(), testdata.Value, nested.Value, api.Value, legacy.Value, apiclient.Value)
}
