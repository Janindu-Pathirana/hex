package util

import (
	"encoding/json"
	"fmt"
	"log"
)

func PrettyPrintJSON(object interface{}) string {
	b, err := json.MarshalIndent(object, "", "  ")
	if err != nil {
		log.Fatalf("Error marshalling to JSON: %v", err)
	}

	fmt.Println(string(b))
	return string(b)
}
