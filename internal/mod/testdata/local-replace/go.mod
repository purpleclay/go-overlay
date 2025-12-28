module example.com/local-replace-test

go 1.22

require example.com/localmod v0.0.0

replace example.com/localmod => ./localmod
