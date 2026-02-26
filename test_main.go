package main

import "fmt"

func main() {
	result := add(1, 2)
	fmt.Println("Result:", result)
}

func add(a, b int) int {
	return a + b
}

func multiply(a, b int) int {
	return a * b
}

func divide(a, b int) int {
	if b == 0 {
		return 0
	}
	return a / b
}
