package goredis

import (
	"fmt"
)

func main() {

	// Initialize connection
	msg, err := Connect("127.0.0.1", 7379)
	if err != nil {
		fmt.Println("Connection error:", err)
		return
	}
	fmt.Println(msg)

	// Authenticate if required
	authRes, err := Auth("root", "dsl")
	if err != nil {
		fmt.Println("Auth error:", err)
	} else {
		fmt.Println("Auth:", authRes)
	}

	// Select DB and perform operations
	Select(0)
	Set("my_key", "Hello from Go!")
	result, _ := Get("my_key")
	fmt.Println("Get:", result)

	delRes, _ := Del("my_key")
	fmt.Println("Del:", delRes)

	getAfterDel, _ := Get("my_key")
	fmt.Println("Get after delete:", getAfterDel)

	// List operations
	LPush("my_list", "item1", "item2")
	listRes, _ := LGet("my_list")
	fmt.Println("List:", listRes)

	// Transaction example
	Watch("my_key")
	Multi()
	Set("my_key", "new_value")
	txRes, _ := Exec()
	fmt.Println("Transaction Results:", txRes)

	// Close connection
	Close()
}
