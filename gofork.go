package main

import (
	"container/list"
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
			ahead := list.New()
			behind := list.New()
			even := list.New()
			private := list.New()
			for _, fork := range forks {
				url = "https://api.github.com/repos/" + fork.Full_name + "/compare/" + repo_info.Owner.Login + ":master...master"
				req, _ = http.NewRequest("GET", url, nil)
				req.Header.Add("Authorization", "token "+string(auth.Token))
				resp, _ = http.DefaultClient.Do(req)
				body, _ = ioutil.ReadAll(resp.Body)
				json.Unmarshal(body, &fork)
				if fork.Status == "ahead" {
					ahead.PushBack(fork)
				} else if fork.Status == "behind" {
					behind.PushBack(fork)
				} else if fork.Status == "identical" {
					even.PushBack(fork)
				} else {
					private.PushBack(fork)
				}
			}
			for e := ahead.Front(); e != nil; e = e.Next() {
				color.Green.Println(success, e.Value.(Fork).Full_name, "is", e.Value.(Fork).Ahead_by, "commits ahead ")

			}
			for e := behind.Front(); e != nil; e = e.Next() {
				color.Red.Println(fail, e.Value.(Fork).Full_name, "is", e.Value.(Fork).Behind_by, "commits behind ")
			}
			for e := even.Front(); e != nil; e = e.Next() {
				color.Yellow.Println(fail, e.Value.(Fork).Full_name, "is up to date")
			}
			for e := private.Front(); e != nil; e = e.Next() {
				color.Blue.Println(fail, e.Value.(Fork).Full_name, "is private")
			}
		}
	}
}
