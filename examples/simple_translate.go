//go:build ignore
// +build ignore

package main

import (
	"fmt"
	translator "github.com/mudita33/go-googletrans"
)

var content = `你好，世界！`

func main() {
	agent := true
	c := translator.Config{
		Proxy:                 []string{"http://127.0.0.1:8080"},
		UserAgent:             nil,
		ServiceUrls:           []string{"translate.google.com.hk"},
		UseUserAgentGenerator: &agent,
	}
	t := translator.New(c)
	client := t.Client()
	result, err := client.Translate(content, "auto", "en")
	if err != nil {
		panic(err)
	}
	fmt.Println(result.Text)
}
