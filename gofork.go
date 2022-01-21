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
	Owner      Owner
}

type Owner struct {
	Login string `json:"login"`
}

type Fork struct {
	Full_name   string `json:"full_name"`
	Compare_url string `json:"compare_url"`
	Status      string `json:"status"`
	Ahead_by    int    `json:"ahead_by"`
	Behind_by   int    `json:"behind_by"`
}

type Auth struct {
	Token string `json:"PAT"`
}

func main() {
	var (
		repo_info Repo_info
		forks     []Fork
		auth      Auth
	)
	dat, _ := os.ReadFile("./config.json")
	json.Unmarshal([]byte(dat), &auth)
	fail := "[X]"
	success := "[✓]"
	working := "[+]"
	repo := os.Args[1]
	fmt.Println(working + " Looking for " + repo)
	url := "https://api.github.com/repos/" + repo
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", "token "+string(auth.Token))
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode == http.StatusNotFound {
		color.Red.Println(fail + " Repository not found")
	} else if resp.StatusCode == http.StatusForbidden {
		color.Red.Println(fail + " No PAT")
	} else {
		color.Green.Println(success + " Repository found")
		body, _ := ioutil.ReadAll(resp.Body)
		json.Unmarshal(body, &repo_info)
		if repo_info.Fork_count == 0 {
			color.Red.Println(fail + " No forks found")
		} else {
			color.Green.Println(success, repo_info.Fork_count, "Forks found")
			url = "https://api.github.com/repos/" + repo + "/forks"
			req, _ = http.NewRequest("GET", url, nil)
			req.Header.Add("Authorization", "token "+string(auth.Token))
			resp, _ = http.DefaultClient.Do(req)
			body, _ = ioutil.ReadAll(resp.Body)
			json.Unmarshal(body, &forks)
			for _, fork := range forks {
				url = "https://api.github.com/repos/" + fork.Full_name + "/compare/" + repo_info.Owner.Login + ":master...master"
				req, _ = http.NewRequest("GET", url, nil)
				req.Header.Add("Authorization", "token "+string(auth.Token))
				resp, _ = http.DefaultClient.Do(req)
				body, _ = ioutil.ReadAll(resp.Body)
				json.Unmarshal(body, &fork)
				if fork.Status == "ahead" {
					color.Green.Println(success, fork.Full_name, "ahead by", fork.Ahead_by, "commits")
				} else if fork.Status == "behind" {
					color.Red.Println(fail, fork.Full_name, "behind by", fork.Behind_by, "commits")
				} else if fork.Status == "identical" {
					color.Yellow.Println(fail, fork.Full_name, "up to date")
				} else {
					color.Blue.Println(fail, fork.Full_name, "Fork is private")
				}
			}
		}
	}
}
