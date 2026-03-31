package main

import "math/rand/v2"

func main() {
	printer.Println(quotes[rand.IntN(len(quotes))])
}
