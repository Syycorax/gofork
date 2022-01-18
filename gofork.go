package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/buger/jsonparser"
	"github.com/gookit/color"
)

func main() {
	fail := "[X]"
	success := "[✓]"
	working := "[+]"
	repo := os.Args[1]
	fmt.Println(working + " Looking for " + repo)
	url := "https://api.github.com/repos/" + repo
	req, _ := http.NewRequest("GET", url, nil)
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode == http.StatusNotFound {
		color.Red.Println(fail + " Repository not found")
	} else {
		color.Green.Println(success + " Repository found")
		body, _ := ioutil.ReadAll(resp.Body)
		fork_count, _ := jsonparser.GetInt(body, "forks_count")
		if fork_count == 0 {
			color.Red.Println(fail + " No forks found")
		} else {
			color.Green.Println(success, fork_count, "Forks found")
		}
	}
}
