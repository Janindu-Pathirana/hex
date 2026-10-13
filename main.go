package main

import (
	_ "time/tzdata" // Asia/Colombo must resolve on machines without a zoneinfo database

	"github.com/Janindu-Pathirana/hex/cmd"
)

func main() {
	cmd.Execute()
}
