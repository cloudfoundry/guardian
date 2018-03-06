package main

import "time"

func main() {
	for {
		time.Sleep(time.Hour * 24 * 365)
	}
}
