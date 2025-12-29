module example.com/api

go 1.22

require (
	example.com/shared v0.0.0
	github.com/aymanbagabas/go-udiff v0.2.0
)

replace example.com/shared => ../shared
