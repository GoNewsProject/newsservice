package main

import "newsservice/internal/app"

func main() {
	err := app.Run()
	if err != nil {
		panic(err)
	}
}
