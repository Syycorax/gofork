package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/gookit/color"
)

type Repo_info struct {
	Fork_count int `json:"forks_count"`
	login      Owner
}

type Owner struct {
	Login string `json:"login"`
}

type Fork struct {
	Full_name   string `json:"full_name"`
	Compare_url string `json:"compare_url"`
	Status      string `json:"status"`
	Ahead_by    int    `json:"ahead_by"`
}

func main() {
	var (
		repo_info Repo_info
		forks     []Fork
	)

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
		json.Unmarshal(body, &repo_info)
		if repo_info.Fork_count == 0 {
			color.Red.Println(fail + " No forks found")
		} else {
			fmt.Println(repo_info.login)
			color.Green.Println(success, repo_info.Fork_count, "Forks found")
			url = "https://api.github.com/repos/" + repo + "/forks"
			req, _ = http.NewRequest("GET", url, nil)
			resp, _ = http.DefaultClient.Do(req)
			body, _ = ioutil.ReadAll(resp.Body)
			json.Unmarshal(body, &forks)
			for _, fork := range forks {
				url = "https://api.github.com/repos/" + fork.Full_name + "/compare/master...develop"
				fmt.Println(fork.Full_name)
			}
		}
	}
}
