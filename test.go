package main

import "fmt"

func add(x int, y int) (int, int) {
var ad, sub int
ad = x + y
sub = x-y
return ad, sub
}

func something() {
	fmt.Println(add(42, 13))
}

