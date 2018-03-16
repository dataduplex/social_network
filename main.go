package main

import "fmt"

func main() {
	a := App{}
	// You need to set your Username and Password here
	a.Initialize("root", "MyNewPass", "social_net")

	fmt.Println("Running the server")

	a.Run(":8080")
}
