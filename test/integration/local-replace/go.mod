module example.com/integration-local-replace

go 1.22

require example.com/localmod v0.0.0

replace example.com/localmod => ./localmod
