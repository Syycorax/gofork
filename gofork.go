package main

import (
	"container/list"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	"github.com/akamensky/argparse"
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
	fail := "[X]"
	success := "[✓]"
	working := "[+]"
	mitigate := "[~]"
	parser := argparse.NewParser("gofork", "CLI tool to find active forks")
	repo := parser.String("r", "repo", &argparse.Options{Required: true, Help: "Repository to check"})
	branch := parser.String("b", "branch", &argparse.Options{Required: false, Help: "Branch to check", Default: "master"})
	err := parser.Parse(os.Args)
	if err != nil {
		fmt.Print(parser.Usage(err))
	}
	fmt.Println(*repo)
	fmt.Println(*branch)
	dat, _ := os.ReadFile("./config.json")
	json.Unmarshal([]byte(dat), &auth)
	fmt.Println(working + " Looking for " + *repo + " on branch " + *branch)
	url := "https://api.github.com/repos/" + *repo
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", "token "+string(auth.Token))
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode == http.StatusNotFound {
		color.Red.Println(fail + " Repository not found")
	} else if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		color.Red.Printf(fail + " Incorrect PAT or no PAT provided (see config.json.example)")
	} else {
		color.Green.Println(success + " Repository found")
		body, _ := ioutil.ReadAll(resp.Body)
		json.Unmarshal(body, &repo_info)
		if repo_info.Fork_count == 0 {
			color.Red.Println(fail + " No forks found")
		} else {
			color.Green.Println(success, repo_info.Fork_count, "Forks found")
			url = "https://api.github.com/repos/" + *repo + "/forks?per_page=" + strconv.Itoa(repo_info.Fork_count)
			req, _ = http.NewRequest("GET", url, nil)
			req.Header.Add("Authorization", "token "+string(auth.Token))
			resp, _ = http.DefaultClient.Do(req)
			body, _ = ioutil.ReadAll(resp.Body)
			json.Unmarshal(body, &forks)
			ahead := list.New()
			behind := list.New()
			diverge := list.New()
			even := list.New()
			private := list.New()
			for _, fork := range forks {
				url = "https://api.github.com/repos/" + fork.Full_name + "/compare/" + repo_info.Owner.Login + ":" + *branch + "..." + *branch
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
				} else if fork.Status == "diverged" {
					diverge.PushBack(fork)
				} else {
					private.PushBack(fork)
				}
			}
			for e := ahead.Front(); e != nil; e = e.Next() {
				color.Green.Println(success, e.Value.(Fork).Full_name, "is", e.Value.(Fork).Ahead_by, "commits ahead ")

			}
			for e := diverge.Front(); e != nil; e = e.Next() {
				color.HEX("#ff2a00").Println(mitigate, e.Value.(Fork).Full_name, "is", e.Value.(Fork).Ahead_by, "commits ahead and", e.Value.(Fork).Behind_by, "commits behind")
			}
			for e := behind.Front(); e != nil; e = e.Next() {
				color.Red.Println(fail, e.Value.(Fork).Full_name, "is", e.Value.(Fork).Behind_by, "commits behind ")
			}
			for e := even.Front(); e != nil; e = e.Next() {
				color.Yellow.Println(fail, e.Value.(Fork).Full_name, "is up to date")
			}
			for e := private.Front(); e != nil; e = e.Next() {
				color.Blue.Println(fail, e.Value.(Fork).Full_name, "has no branch master or is private")
			}
		}
	}
}
