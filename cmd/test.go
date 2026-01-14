package main

import (
	"fmt"

	grs "github.com/akashmaji946/go-redis/go-client"
)

func foo() {
	// Initialize connection
	msg, err := grs.Connect("127.0.0.1", 7379)
	if err != nil {
		fmt.Println("Connection error:", err)
		return
	}
	fmt.Println(msg)

	// Authenticate if required
	authRes, err := grs.Auth("root", "dsl")
	if err != nil {
		fmt.Println("Auth error:", err)
	} else {
		fmt.Println("Auth:", authRes)
	}

	// Select DB and perform operations
	grs.Select(0)
	grs.Set("my_key", "Hello from Go!")
	result, _ := grs.Get("my_key")
	fmt.Println("Get:", result)

	delRes, _ := grs.Del("my_key")
	fmt.Println("Del:", delRes)

	getAfterDel, _ := grs.Get("my_key")
	fmt.Println("Get after delete:", getAfterDel)

	// List operations
	grs.LPush("my_list", "item1", "item2")
	listRes, _ := grs.LGet("my_list")
	fmt.Println("List:", listRes)

	// Transaction example
	grs.Watch("my_key")
	grs.Multi()
	grs.Set("my_key", "new_value")
	txRes, _ := grs.Exec()
	fmt.Println("Transaction Results:", txRes)

	// Close connection
	grs.Close()

}
