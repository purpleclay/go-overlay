module example.com/nested-app

go 1.24.0

require (
	example.com/nested v0.0.0
	example.com/nested/api v0.0.0
	github.com/go-ini/ini v1.66.6
	github.com/go-ini/ini/testdata v0.0.0
)

replace github.com/go-ini/ini => gopkg.in/ini.v1 v1.67.3
replace github.com/go-ini/ini/testdata => ../child
replace example.com/nested => ../parent
replace example.com/nested/api => ../api
