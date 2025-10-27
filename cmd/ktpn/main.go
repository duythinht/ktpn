package main

import (
	"encoding/json"
	"fmt"
	_ "image/jpeg"
	"log"
	"os"

	"github.com/duythinht/ktpn"
)

func main() {
	result, err := ktpn.Retry(5, func() ([]ktpn.Violation, error) {
		return ktpn.KT(os.Args[1], ktpn.Car)
	})
	if err != nil {
		log.Fatal(err)
	}

	out, err := json.Marshal(result)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s\n", out)
}
