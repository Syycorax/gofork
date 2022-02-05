package main

import (
	"container/list"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"strconv"

	"github.com/akamensky/argparse"
	"github.com/gookit/color"
)

type Repo_info struct {
	Fork_count     int `json:"forks_count"`
	Owner          Owner
	Default_branch string `json:"default_branch"`
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
		os.Exit(1)
	}
	platform := runtime.GOOS
	dat, _ := os.ReadFile("./config.json")
	json.Unmarshal([]byte(dat), &auth)
	color.Notice.Println(working + " Looking for " + *repo + " on branch " + *branch)
	url := "https://api.github.com/repos/" + *repo
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", "token "+string(auth.Token))
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode == http.StatusNotFound {
		if platform == "windows" {
			color.Error.Print(fail + " Repository not found")
		} else {
			color.Error.Print(fail + " Repository not found\n")
		}
	} else if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		if platform == "windows" {
			color.Error.Print(fail + " Incorrect PAT or no PAT provided (see config.json.example)")
		} else {
			color.Error.Print(fail + " Incorrect PAT or no PAT provided (see config.json.example)\n")
		}
	} else {
		color.Success.Println(success + " Repository found")
		body, _ := ioutil.ReadAll(resp.Body)
		json.Unmarshal(body, &repo_info)
		if repo_info.Fork_count == 0 {
			if platform == "windows" {
				color.Error.Print(fail + " No forks found")
			} else {
				color.Error.Print(fail + " No forks found \n")
			}
		} else {
			color.Success.Println(success, repo_info.Fork_count, "Forks found")
			if repo_info.Fork_count > 100 {
				color.Info.Println(mitigate + " More than 100 forks found, only showing first 100")
			}
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
				color.Success.Println(success, e.Value.(Fork).Full_name, "is", e.Value.(Fork).Ahead_by, "commits ahead ")
			}
			for e := diverge.Front(); e != nil; e = e.Next() {
				color.Warn.Println(mitigate, e.Value.(Fork).Full_name, "is", e.Value.(Fork).Ahead_by, "commits ahead and", e.Value.(Fork).Behind_by, "commits behind")
			}
			for e := behind.Front(); e != nil; e = e.Next() {
				color.Error.Println(fail, e.Value.(Fork).Full_name, "is", e.Value.(Fork).Behind_by, "commits behind ")
			}
			for e := even.Front(); e != nil; e = e.Next() {
				color.Danger.Println(fail, e.Value.(Fork).Full_name, "is up to date")
			}
			for e := private.Front(); e != nil; e = e.Next() {
				color.Question.Println(fail, e.Value.(Fork).Full_name, "has no branch "+*branch+" or is private")
			}
			if ahead.Len() == 0 && behind.Len() == 0 && even.Len() == 0 && *branch == "master" {
				color.Error.Println(fail, "No forks found on branch master, maybe try with main?")
			}
		}
	}
}
