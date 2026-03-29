module example.com/remote-replace

go 1.25.4

require github.com/go-ini/ini v1.67.0

require github.com/stretchr/testify v1.11.1 // indirect

replace github.com/go-ini/ini => gopkg.in/ini.v1 v1.67.0
