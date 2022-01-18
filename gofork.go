package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/gookit/color"
)

type Repo_info struct {
	Forks_count int    `json:"forks_count"`
	Fork_url    string `json:"forks_url"`
}

type Fork_info struct {
	Forks []struct {
		Fork_url  string `json:"html_url"`
		Author    string `json:"owner.login"`
		Full_name string `json:"full_name"`
	}
}

func Error(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	var (
		repo_info Repo_info
		fork_info Fork_info
		//forks Fork_info
	)
	fail := "[X]"
	success := "[✓]"
	working := "[+]"
	repo := os.Args[1]
	fmt.Println(working + " Looking for " + repo)
	url := "https://api.github.com/repos/" + repo
	req, _ := http.NewRequest("GET", url, nil)
	resp, err := http.DefaultClient.Do(req)
	Error(err)
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	json_response := string(body)
	json.Unmarshal([]byte(json_response), &repo_info)
	if resp.StatusCode == http.StatusNotFound {
		color.Red.Println(fail + " Repository not found")
	} else {
		color.Green.Println(success + " Repository found")
		if repo_info.Forks_count == 0 {
			color.Red.Println(fail + " No forks found")
		} else {
			color.Green.Println(success, repo_info.Forks_count, "Forks found")
			url = "http://api.github.com/repos/" + repo + "/forks"
			req, _ = http.NewRequest("GET", url, nil)
			resp, err = http.DefaultClient.Do(req)
			Error(err)
			defer resp.Body.Close()
			body, _ = ioutil.ReadAll(resp.Body)
			json_response = string(body)
			fmt.Printf("%s\n", json_response)
			json.Unmarshal([]byte(json_response), &fork_info)
			f, _ := os.Create("data.txt")
			defer f.Close()
			_,_ = f.WriteString(string(json_response))
			for range fork_info.Forks {
				//fmt.Printf("%s %s %s\n", success, forks.Author, forks.Fork_url)
			}

		}
	}

}