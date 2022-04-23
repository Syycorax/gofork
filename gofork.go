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
		platformPrint(color.Warn, parser.Usage(err))
		os.Exit(1)
	}
	auth.Token = readConfig()
	if auth.Token == "" {
		// TODO: don't store token in plaintext
		platformPrint(color.Error, "Please provide a PAT (https://tinyurl.com/GITHUBPAT) (Don't allow any scope, the token is stored in PLAINTEXT)")
		input := getInput()
		output := "{\"PAT\": \"" + input + "\"}"
		writeConfig(output)
		platformPrint(color.Success, "PAT saved")
		auth.Token = readConfig()
	}
	if *repo == "" {
		platformPrint(color.Error, "Please provide a repository")
		os.Exit(1)
	}
	platformPrint(color.Notice, working+" Looking for "+*repo)
	if RepoCheck(*repo, auth.Token) == 1 {
		platformPrint(color.Error, fail+" Repository not found")
		os.Exit(1)
	} else if RepoCheck(*repo, auth.Token) == 2 {
		platformPrint(color.Error, fail+" Incorrect PAT, do you want to delete config file? (y/n)")
		input := getInput()
		if input == "y" || input == "Y" {
			deleteConfig()
			platformPrint(color.Success, "PAT deleted")
			os.Exit(1)
		} else if input == "n" || input == "N" {
			platformPrint(color.Error, "Incorrect PAT provided exiting")
			os.Exit(1)
		} else {
			platformPrint(color.Error, "Incorrect input provided exiting")
			os.Exit(1)
		}
	} else if RepoCheck(*repo, auth.Token) == 3 {
		platformPrint(color.Error, fail+" Unknow error")
	} else {
		platformPrint(color.Success, success+" Found "+*repo)
		RepoInfo = getRepoInfo(*repo, auth.Token)
		if *branch == "repo default branch" {
			platformPrint(color.Notice, working+" No branch provided, using default branch")
			*branch = RepoInfo.DefaultBranch
		}
		platformPrint(color.Notice, working+" Looking for "+*repo+":"+*branch)
		if RepoInfo.ForkCount == 0 {
			platformPrint(color.Error, fail+" No forks found")
		} else {
			platformPrint(color.Success, success+" "+strconv.Itoa(RepoInfo.ForkCount)+" Forks found")
			pagesDecimal := float64(RepoInfo.ForkCount) / float64(100)
			// The total number of pages
			pages := RepoInfo.ForkCount / 100
			if pagesDecimal != float64(int(pagesDecimal)) {
				pages = int(pages) + 1
			}
			if *page > pages {
				platformPrint(color.Warn, warning+" The page is out of range (max. "+strconv.Itoa(pages)+"), showing page 1")
				*page = 1
			}
			if RepoInfo.ForkCount > 100 && *page == 1 {
				RepoInfo.ForkCount = 100
				// Force the loop to iterate over the selected page only
				pages = *page
				platformPrint(color.Info, mitigate+" More than 100 forks found, only showing first 100 (use -p to get other results)")
			}
			if RepoInfo.ForkCount > 100 && *page > 1 {
				RepoInfo.ForkCount = 100
				// Force the loop to iterate over the selected page only
				pages = *page
				platformPrint(color.Info, mitigate+" More than 100 forks found, showing page "+strconv.Itoa(*page))
			}
			if RepoInfo.ForkCount > 100 && *page == -1 {
				platformPrint(color.Info, mitigate+" More than 100 forks found, showing page all pages because -p is used with -1")
			}
			if *page < 1 {
				if *page != -1 {
					platformPrint(color.Warn, warning+" The number of page is lower than 1, showing page 1")
					pages = 1
					RepoInfo.ForkCount = 100
				}
				*page = 1
			}
			ahead := list.New()
			behind := list.New()
			diverge := list.New()
			even := list.New()
			private := list.New()
			bar := progressbar.Default(int64(RepoInfo.ForkCount))
			for page := *page; page < pages+1; page++ {
				url := "https://api.github.com/repos/" + *repo + "/forks?per_page=" + strconv.Itoa(RepoInfo.ForkCount)
				url = url + "&page=" + strconv.Itoa(page)
				req, _ := http.NewRequest("GET", url, nil)
				req.Header.Add("Authorization", "token "+string(auth.Token))
				resp, _ := http.DefaultClient.Do(req)
				body, _ := ioutil.ReadAll(resp.Body)
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
			sortTable(ahead, "desc")
			sortTable(behind, "asc")
			sortTable(diverge, "desc")
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
				platformPrint(color.Success, success+" Forks ahead:")
				aheadtable.Render()
			} else {
				platformPrint(color.Notice, mitigate+" No forks ahead of "+RepoInfo.Owner.Login+":"+*branch)
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
				platformPrint(color.Notice, mitigate+" Forks diverged:")
				divergetable.Render()
			} else {
				platformPrint(color.Notice, mitigate+" No forks diverged of "+RepoInfo.Owner.Login+":"+*branch)
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
				platformPrint(color.Warn, fail+" Forks behind:")
				behindtable.Render()
			} else {
				platformPrint(color.Notice, mitigate+" No forks behind of "+RepoInfo.Owner.Login+":"+*branch)
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
					platformPrint(color.Notice, mitigate+" Forks up to date:")
					eventable.Render()
				} else {
					platformPrint(color.Notice, mitigate+" No forks identical to "+RepoInfo.Owner.Login+":"+*branch)
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
					platformPrint(color.Question, mitigate+" Private forks:")
					privatetable.Render()
				} else {
					platformPrint(color.Notice, mitigate+" No forks private of "+RepoInfo.Owner.Login+":"+*branch)
				}
			}
			if ahead.Len() == 0 && behind.Len() == 0 && even.Len() == 0 && diverge.Len() == 0 && *branch == "master" {
				platformPrint(color.Error, fail+" No forks found on branch master maybe try with main?")
			}
		}
	}
}
func getConfigFilePath() (string, string) {
	//get the config file path depending on the OS
	var (
		ConfigFilePath string
		path           string
	)
	if runtime.GOOS == "windows" {
		path, _ = os.UserConfigDir()
		path = path + "\\gofork"
		ConfigFilePath = path + "\\config.json"
	} else {
		path = os.Getenv("HOME") + "/.config/gofork/"
		ConfigFilePath = path + "gofork.conf"
	}
	return path, ConfigFilePath
}

func readConfig() string {
	var (
		auth Auth
	)
	//read the config file
	_, configFilePath := getConfigFilePath()
	dat, _ := os.ReadFile(configFilePath)
	json.Unmarshal([]byte(dat), &auth)
	return auth.Token
}
func writeConfig(token string) {
	//write the token to the config file depending on the OS
	path, cfp := getConfigFilePath()
	if runtime.GOOS == "windows" {
		os.Mkdir(path, 0777)
		ioutil.WriteFile(cfp, []byte(token), 0644)
		platformPrint(color.Success, "Token written to config file "+cfp)
	} else {
		os.MkdirAll(path, 0777)
		ioutil.WriteFile(cfp, []byte(token), 0644)
		platformPrint(color.Success, "Token written to config file "+cfp)

	}
}
func deleteConfig() {
	_, configFilePath := getConfigFilePath()
	os.Remove(configFilePath)

}
func parseInput(data string) string {
	// parses the user input depending on the OS
	platform := runtime.GOOS
	if platform == "windows" {
		data = strings.Replace(data, "\r\n", "", -1)
	} else {
		data = strings.Replace(data, "\n", "", -1)
	}
	return data

}
func getInput() string {
	// get token from input and parses it with ParseInput()
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = parseInput(input)
	return input
}
func RepoCheck(repo string, token string) int {
	// checks if the repo is a valid github repo
	url := "https://api.github.com/repos/" + repo
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", "token "+token)
	res, _ := http.DefaultClient.Do(req)
	if res.StatusCode == 200 {
		return 0
	} else if res.StatusCode == 404 {
		return 1
	} else if res.StatusCode == 401 {
		return 2
	} else {
		return 3
	}
}
func getRepoInfo(repo string, token string) RepoInfo {
	// gets the repo info from github
	var (
		repoInfo RepoInfo
	)
	url := "https://api.github.com/repos/" + repo
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", "token "+token)
	res, _ := http.DefaultClient.Do(req)
	body, _ := ioutil.ReadAll(res.Body)
	json.Unmarshal(body, &repoInfo)
	return repoInfo
}
func platformPrint(c *color.Theme, text string) {
	// prints the text depending on the OS
	platform := runtime.GOOS
	if platform == "windows" {
		color.Theme(*c).Println(text)
	} else {
		color.Theme(*c).Println(text)
	}
}
func sortTable(table *list.List, order string) list.List {
	if order == "desc" {
		for e := table.Front(); e != nil; e = e.Next() {
			for f := e.Next(); f != nil; f = f.Next() {
				if e.Value.(Fork).AheadBy < f.Value.(Fork).AheadBy {
					e.Value, f.Value = f.Value, e.Value
				}
			}
		}
	} else {
		for e := table.Front(); e != nil; e = e.Next() {
			for f := e.Next(); f != nil; f = f.Next() {
				if e.Value.(Fork).BehindBy > f.Value.(Fork).BehindBy {
					e.Value, f.Value = f.Value, e.Value
				}
			}
		}
	}
	return *table
}
