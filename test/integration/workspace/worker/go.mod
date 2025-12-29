module example.com/worker

go 1.22

require (
	example.com/shared v0.0.0
	github.com/fatih/color v1.18.0
)

require (
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	golang.org/x/sys v0.25.0 // indirect
)

replace example.com/shared => ../shared
