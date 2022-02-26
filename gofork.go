package main

import (
	"bufio"
	"container/list"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/akamensky/argparse"
	"github.com/gookit/color"
	"github.com/olekukonko/tablewriter"
	"github.com/schollz/progressbar/v3"
)

type RepoInfo struct {
	ForkCount     int `json:"forks_count"`
	Owner         Owner
	DefaultBranch string `json:"default_branch"`
}

type Owner struct {
	Login string `json:"login"`
}

type Fork struct {
	FullName string `json:"full_name"`
	Url      string `json:"html_url"`
	Status   string `json:"status"`
	AheadBy  int    `json:"ahead_by"`
	BehindBy int    `json:"behind_by"`
}

type Auth struct {
	Token string `json:"PAT"`
}

func main() {
	var (
		RepoInfo RepoInfo
		forks    []Fork
		auth     Auth
	)
	fail := "[X]"
	success := "[✓]"
	warning := "[!]"
	working := "[+]"
	mitigate := "[~]"
	parser := argparse.NewParser("gofork", "CLI tool to find active forks")
	repo := parser.String("r", "repo", &argparse.Options{Required: false, Help: "Repository to check"})
	branch := parser.String("b", "branch", &argparse.Options{Required: false, Help: "Branch to check", Default: "repo default branch"})
	verboseflag := parser.Flag("v", "verbose", &argparse.Options{Help: "Show private and up to date repositories"})
	page := parser.Int("p", "page", &argparse.Options{Help: "Page to check (use -1 for all)", Default: 1, Required: false})
	err := parser.Parse(os.Args)
	if err != nil {
		color.Error.Println(parser.Usage(err))
		os.Exit(1)
	}
	platform := runtime.GOOS
	dat, _ := os.ReadFile("./config.json")
	json.Unmarshal([]byte(dat), &auth)
	if auth.Token == "" {
		color.Error.Println("Please provide a PAT (https://tinyurl.com/GITHUBPAT) (no scope required)")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		if platform == "windows" {
			input = strings.Replace(input, "\r\n", "", -1)
		} else {
			input = strings.Replace(input, "\n", "", -1)
		}
		output := "{\"PAT\": \"" + input + "\"}"
		ioutil.WriteFile("./config.json", []byte(output), 0644)
		color.Success.Println("PAT saved")
		dat, _ := os.ReadFile("./config.json")
		json.Unmarshal([]byte(dat), &auth)
	}
	if *repo == "" {
		color.Error.Println("Please provide a repository")
		os.Exit(1)
	}
	color.Notice.Println(working + " Looking for " + *repo)
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
		color.Error.Println(fail + " Incorrect PAT, do you want to delete config file? (y/n)")

		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		if platform == "windows" {
			input = strings.Replace(input, "\r\n", "", -1)
		} else {
			input = strings.Replace(input, "\n", "", -1)
		}
		if input == "y" {
			os.Remove("./config.json")
			color.Success.Println("PAT deleted")
			os.Exit(1)
		} else {
			if platform == "windows" {
				color.Error.Println("Incorrect PAT provided exiting")
			} else {
				color.Error.Print("Incorrect PAT provided exiting\n")
			}
			os.Exit(1)
		}
	} else {
		color.Success.Println(success + " Repository found")
		body, _ := ioutil.ReadAll(resp.Body)
		json.Unmarshal(body, &RepoInfo)
		if *branch == "repo default branch" {
			*branch = RepoInfo.DefaultBranch
		}
		color.Notice.Println(working + " Looking for " + *repo + ":" + *branch)
		if RepoInfo.ForkCount == 0 {
			if platform == "windows" {
				color.Error.Print(fail + " No forks found")
			} else {
				color.Error.Print(fail + " No forks found \n")
			}
		} else {
			color.Success.Println(success, RepoInfo.ForkCount, "Forks found")
			// Temporary variable to avoid avoid repetitions
			pagesDecimal := float64(RepoInfo.ForkCount) / float64(100)
			// The total number of pages
			pages := RepoInfo.ForkCount / 100
			if pagesDecimal != float64(int(pagesDecimal)) {
				pages = int(pages) + 1
			}
			if *page > pages {
				color.Warn.Println(warning + " The page is out of range (max. " + strconv.Itoa(pages) + "), showing page 1")
				*page = 1
			}
			// This is separate from the above if because of it crashes if there is less than 100 forks
			if *page < 1 {
				if *page != -1 {
					color.Warn.Println(warning + " The number of page is lower than 1, showing page 1")
					pages = 1
					RepoInfo.ForkCount = 100
				}
				*page = 1
			}
			if RepoInfo.ForkCount > 100 && *page == 1 {
				RepoInfo.ForkCount = 100
				// Force the loop to iterate over the selected page only
				pages = *page
				color.Info.Println(mitigate + " More than 100 forks found, only showing first 100 (use -p to get other results)")
			}
			if RepoInfo.ForkCount > 100 && *page > 1 {
				RepoInfo.ForkCount = 100
				// Force the loop to iterate over the selected page only
				pages = *page
				color.Info.Println(mitigate + " More than 100 forks found, showing page " + strconv.Itoa(*page))
			}
			if RepoInfo.ForkCount > 100 && *page == -1 {
				color.Info.Println(mitigate + " More than 100 forks found, showing page all pages because -p is used with -1")
			}
			ahead := list.New()
			behind := list.New()
			diverge := list.New()
			even := list.New()
			private := list.New()
			bar := progressbar.Default(int64(RepoInfo.ForkCount))
			for page := *page; page < pages+1; page++ {
				url = "https://api.github.com/repos/" + *repo + "/forks?per_page=" + strconv.Itoa(RepoInfo.ForkCount)
				url = url + "&page=" + strconv.Itoa(page)
				req, _ = http.NewRequest("GET", url, nil)
				req.Header.Add("Authorization", "token "+string(auth.Token))
				resp, _ = http.DefaultClient.Do(req)
				body, _ = ioutil.ReadAll(resp.Body)
				json.Unmarshal(body, &forks)
				for _, fork := range forks {
					url = "https://api.github.com/repos/" + fork.FullName + "/compare/" + RepoInfo.Owner.Login + ":" + *branch + "..." + *branch
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
					bar.Add(1)
				}
			}
			// sort ahead by ahead_by descending
			for e := ahead.Front(); e != nil; e = e.Next() {
				for f := e.Next(); f != nil; f = f.Next() {
					if e.Value.(Fork).AheadBy < f.Value.(Fork).AheadBy {
						e.Value, f.Value = f.Value, e.Value
					}
				}
			}
			// sort behind by behind_by ascending
			for e := behind.Front(); e != nil; e = e.Next() {
				for f := e.Next(); f != nil; f = f.Next() {
					if e.Value.(Fork).BehindBy > f.Value.(Fork).BehindBy {
						e.Value, f.Value = f.Value, e.Value
					}
				}
			}
			// sort diverge by ahead_by descending
			for e := diverge.Front(); e != nil; e = e.Next() {
				for f := e.Next(); f != nil; f = f.Next() {
					if e.Value.(Fork).AheadBy < f.Value.(Fork).AheadBy {
						e.Value, f.Value = f.Value, e.Value
					}
				}
			}
			aheadtable := tablewriter.NewWriter(os.Stdout)
			aheadtable.SetHeader([]string{"Fork", "Ahead by", "URL"})
			aheadmap := [][]string{}
			for e := ahead.Front(); e != nil; e = e.Next() {
				fork := e.Value.(Fork)
				aheadBy := strconv.Itoa(fork.AheadBy)
				url := "https://github.com/" + string(fork.FullName)
				aheadmap = append(aheadmap, []string{fork.FullName, aheadBy, url})

			}
			for _, v := range aheadmap {
				aheadtable.Append(v)
			}
			if ahead.Len() > 0 {
				color.Success.Println(success + " Forks ahead:")
				aheadtable.Render()
			} else {
				color.Notice.Println(mitigate + " No forks ahead of " + RepoInfo.Owner.Login + ":" + *branch)
			}
			divergetable := tablewriter.NewWriter(os.Stdout)
			divergetable.SetHeader([]string{"Fork", "Ahead by", "Behind by", "URL"})
			divergemap := [][]string{}
			for e := diverge.Front(); e != nil; e = e.Next() {
				fork := e.Value.(Fork)
				aheadBy := strconv.Itoa(fork.AheadBy)
				behindBy := strconv.Itoa(fork.BehindBy)
				url := "https://github.com/" + string(fork.FullName)
				divergemap = append(divergemap, []string{fork.FullName, aheadBy, behindBy, url})
			}
			for _, v := range divergemap {
				divergetable.Append(v)
			}
			if diverge.Len() > 0 {
				color.Notice.Println(mitigate + " Forks diverged:")
				divergetable.Render()
			} else {
				color.Notice.Println(mitigate + " No forks diverged of " + RepoInfo.Owner.Login + ":" + *branch)
			}
			behindtable := tablewriter.NewWriter(os.Stdout)
			behindtable.SetHeader([]string{"Fork", "Behind by", "URL"})
			behindmap := [][]string{}
			for e := behind.Front(); e != nil; e = e.Next() {
				fork := e.Value.(Fork)
				behindBy := strconv.Itoa(fork.BehindBy)
				url := "https://github.com/" + string(fork.FullName)
				behindmap = append(behindmap, []string{fork.FullName, behindBy, url})
			}
			for _, v := range behindmap {
				behindtable.Append(v)
			}
			if behind.Len() > 0 {
				color.Warn.Println(fail + " Forks behind:")
				behindtable.Render()
			} else {
				color.Notice.Println(mitigate + " No forks behind of " + RepoInfo.Owner.Login + ":" + *branch)
			}
			if *verboseflag {
				eventable := tablewriter.NewWriter(os.Stdout)
				eventable.SetHeader([]string{"Fork", "URL"})
				eventmap := [][]string{}
				for e := even.Front(); e != nil; e = e.Next() {
					fork := e.Value.(Fork)
					url := "https://github.com" + string(fork.FullName)
					eventmap = append(eventmap, []string{fork.FullName, url})
				}
				for _, v := range eventmap {
					eventable.Append(v)
				}
				if even.Len() > 0 {
					color.Notice.Println(mitigate + " Forks up to date:")
					eventable.Render()
				} else {
					color.Notice.Println(mitigate + " No forks identical to " + RepoInfo.Owner.Login + ":" + *branch)
				}
				privatetable := tablewriter.NewWriter(os.Stdout)
				privatetable.SetHeader([]string{"Fork", "URL"})
				privatemap := [][]string{}
				for e := private.Front(); e != nil; e = e.Next() {
					fork := e.Value.(Fork)
					url := "https://github.com" + string(fork.FullName)
					privatemap = append(privatemap, []string{fork.FullName, url})
				}
				for _, v := range privatemap {
					privatetable.Append(v)
				}
				if private.Len() > 0 {
					color.Question.Println(mitigate + " Private forks:")
					privatetable.Render()
				} else {
					color.Notice.Println(mitigate + " No forks private of " + RepoInfo.Owner.Login + ":" + *branch)
				}
			}
			if ahead.Len() == 0 && behind.Len() == 0 && even.Len() == 0 && *branch == "master" {
				color.Error.Println(fail, "No forks found on branch master, maybe try with main?")
			}
		}
	}
}
