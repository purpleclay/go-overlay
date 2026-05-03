module example.com/api

go 1.22

require example.com/shared v0.0.0

require (
	github.com/fatih/color v1.18.0
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/sys v0.25.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace example.com/shared => ../shared
