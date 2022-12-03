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
	"github.com/jedib0t/go-pretty/v6/table"
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

type Branches struct {
	Name string `json:"name"`
}

type Fork struct {
	FullName     string `json:"full_name"`
	Url          string `json:"html_url"`
	Status       string `json:"status"`
	AheadBy      int    `json:"ahead_by"`
	BehindBy     int    `json:"behind_by"`
	Stars        int    `json:"stargazers_count"`
	LastUpdated  string `json:"pushed_at"`
	branchNumber int
}

type Auth struct {
	Token string `json:"PAT"`
}

func main() {
	var (
		forks    []Fork
		auth     Auth
		branches []Branches
	)
	fail := "[X] "
	success := "[✓] "
	warning := "[!] "
	working := "[+] "
	mitigate := "[?] "
	parser := argparse.NewParser("gofork", "CLI tool to find active forks")
	repo := parser.String("r", "repo", &argparse.Options{Required: true, Help: "Repository to check"})
	branch := parser.String("b", "branch", &argparse.Options{Required: false, Help: "Branch to check", Default: "repo default branch"})
	verboseFlag := parser.Flag("v", "verbose", &argparse.Options{Help: "Show deleted and up to date repositories"})
	pageInt := parser.Int("p", "page", &argparse.Options{Help: "Page to check (use -1 for all)", Default: 1, Required: false})
	sortBy := parser.String("s", "sort", &argparse.Options{Help: "Sort by (stars, ahead, lastUpdated, branches)", Default: "ahead", Required: false})
	deleteconfigflag := parser.Flag("d", "deleteconfig", &argparse.Options{Help: "Delete the config file"})
	err := parser.Parse(os.Args)
	if err != nil {
		platformPrint(color.Warn, parser.Usage(err))
		os.Exit(1)
	}
	if *deleteconfigflag {
		err := deleteConfig()
		if err != nil {
			platformPrint(color.Warn, fail+"Error deleting config file")
			os.Exit(1)
		} else {
			platformPrint(color.Success, success+"Config file deleted")
		}
		os.Exit(0)
	}
	auth.Token = readConfig()
	if auth.Token == "" {
		// TODO: don't store token in plaintext
		platformPrint(color.Error, "Please provide a PAT (https://tinyurl.com/GITHUBPAT) (Don't allow any scope, the token is stored in PLAINTEXT)")
		input := getInput()
		output := "{\"PAT\": \"" + input + "\"}"
		writeConfig(output)
		platformPrint(color.Success, "PAT saved to"+getConfigFilePath())
		auth.Token = readConfig()
	}

	platformPrint(color.Notice, working+"Looking for "+*repo)
	if RepoCheck(*repo, auth.Token) == 1 {
		platformPrint(color.Error, fail+"Repository not found")
		os.Exit(1)
	} else if RepoCheck(*repo, auth.Token) == 2 {
		platformPrint(color.Error, fail+"Incorrect PAT, do you want to delete config file? (y/n)")
		input := getInput()
		if input == "y" || input == "Y" {
			err := deleteConfig()
			if err != nil {
				platformPrint(color.Warn, fail+"Error deleting config file")
				os.Exit(1)
			} else {
				platformPrint(color.Success, success+"Config file deleted")
			}
			os.Exit(1)
		} else if input == "n" || input == "N" {
			platformPrint(color.Error, "Incorrect PAT provided exiting")
			os.Exit(1)
		} else {
			platformPrint(color.Error, "Incorrect input provided exiting")
			os.Exit(1)
		}
	} else if RepoCheck(*repo, auth.Token) == 3 {
		platformPrint(color.Error, fail+"Unknow error")

	} else {
		platformPrint(color.Success, success+"Found "+*repo)
		RepoInfo := getRepoInfo(*repo, auth.Token)
		if *branch == "repo default branch" {
			platformPrint(color.Notice, mitigate+"No branch provided, using default branch")
			*branch = RepoInfo.DefaultBranch
		}
		platformPrint(color.Notice, working+"Looking for "+*repo+":"+*branch)
		if RepoInfo.ForkCount == 0 {
			platformPrint(color.Error, fail+"No forks found")

		} else {
			platformPrint(color.Success, success+strconv.Itoa(RepoInfo.ForkCount)+" Forks found")
			pagesDecimal := float64(RepoInfo.ForkCount) / float64(100)
			// The total number of pages
			pages := RepoInfo.ForkCount / 100
			if pagesDecimal != float64(int(pagesDecimal)) {
				pages = int(pages) + 1
			}
			if *pageInt > pages {
				platformPrint(color.Warn, warning+"The page is out of range (max. "+strconv.Itoa(pages)+"), showing page 1")
				*pageInt = 1
			}
			if RepoInfo.ForkCount > 100 && *pageInt == 1 {
				RepoInfo.ForkCount = 100
				// Force the loop to iterate over the selected page only
				pages = *pageInt
				platformPrint(color.Notice, warning+"More than 100 forks found, only showing first 100 (use -p to get other results)")
			}
			if RepoInfo.ForkCount > 100 && *pageInt > 1 {
				RepoInfo.ForkCount = 100
				// Force the loop to iterate over the selected page only
				pages = *pageInt
				platformPrint(color.Notice, warning+"More than 100 forks found, showing page "+strconv.Itoa(*pageInt))
			}
			if RepoInfo.ForkCount > 100 && *pageInt == -1 {
				platformPrint(color.Notice, warning+"More than 100 forks found, showing page all pages because -p is used with -1")
			}
			if *pageInt < 1 {
				if *pageInt != -1 {
					platformPrint(color.Warn, warning+"The number of page is lower than 1, showing page 1")
					pages = 1
					RepoInfo.ForkCount = 100
				}
				*pageInt = 1
			}

			if *sortBy != "stars" && *sortBy != "ahead" && *sortBy != "lastUpdated"  && *sortBy != "branches" {
				platformPrint(color.Warn, warning+"The sort option is not valid, sortingHelper by ahead")
				*sortBy = "ahead"
			}

			ahead, behind, diverge, even, deleted := list.New(), list.New(), list.New(), list.New(), list.New()
			bar := progressbar.Default(int64(RepoInfo.ForkCount))
			for page := *pageInt; page < pages+1; page++ {
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
					url = "https://api.github.com/repos/" + fork.FullName + "/branches"
					req, _ = http.NewRequest("GET", url, nil)
					req.Header.Add("Authorization", "token "+string(auth.Token))
					resp, _ = http.DefaultClient.Do(req)
					body, _ = ioutil.ReadAll(resp.Body)
					json.Unmarshal(body, &branches)
					for range branches {
						fork.branchNumber ++
					}
					if fork.Status == "ahead" {
						ahead.PushBack(fork)
					} else if fork.Status == "behind" {
						behind.PushBack(fork)
					} else if fork.Status == "identical" {
						even.PushBack(fork)
					} else if fork.Status == "diverged" {
						diverge.PushBack(fork)
					} else {
						deleted.PushBack(fork)
					}
					bar.Add(1)
				}
			}
			bar.Finish()
			platformPrint(color.Success, success+"sorting by "+*sortBy)
			aheadTable := table.NewWriter()
			aheadTable.SetOutputMirror(os.Stdout)
			aheadTable.AppendHeader(table.Row{"Fork", "Ahead by", "URL", "Stars","Branches","Last updated"})
			for e := ahead.Front(); e != nil; e = e.Next() {
				fork := e.Value.(Fork)
				aheadBy := strconv.Itoa(fork.AheadBy)
				url := "https://github.com/" + string(fork.FullName)
				stars := strconv.Itoa(fork.Stars) + " \033[33m\u2605" + "\033[0m"
				branches := strconv.Itoa(fork.branchNumber)
				LastUpdated := dateHandler(fork.LastUpdated)
				aheadTable.AppendRow([]interface{}{fork.FullName, aheadBy, url, stars, branches,LastUpdated})
			}
			sortingHelper(aheadTable, sortBy)
			if ahead.Len() > 0 {
				platformPrint(color.Success, success+"Forks ahead: "+strconv.Itoa(ahead.Len()))
				aheadTable.SetStyle(table.StyleRounded)
				aheadTable.Render()
			} else {
				platformPrint(color.Notice, fail+" No forks ahead of "+RepoInfo.Owner.Login+":"+*branch)
			}
			divergeTable := table.NewWriter()
			divergeTable.SetOutputMirror(os.Stdout)
			divergeTable.AppendHeader(table.Row{"Fork", "Ahead by", "Behind by", "URL", "Stars","Branches", "Last Updated"})
			for e := diverge.Front(); e != nil; e = e.Next() {
				fork := e.Value.(Fork)
				aheadBy := strconv.Itoa(fork.AheadBy)
				behindBy := strconv.Itoa(fork.BehindBy)
				url := "https://github.com/" + string(fork.FullName)
				stars := strconv.Itoa(fork.Stars) + " \033[33m\u2605" + "\033[0m"
				branches := strconv.Itoa(fork.branchNumber)
				lastCommit := dateHandler(fork.LastUpdated)
				divergeTable.AppendRow([]interface{}{fork.FullName, aheadBy, behindBy, url, stars, branches, lastCommit})
			}
			sortingHelper(divergeTable, sortBy)
			if diverge.Len() > 0 {
				platformPrint(color.Notice, mitigate+"Forks diverged: "+strconv.Itoa(diverge.Len()))
				divergeTable.SetStyle(table.StyleRounded)
				divergeTable.Render()
			} else {
				platformPrint(color.Notice, fail+"No forks diverged of "+RepoInfo.Owner.Login+":"+*branch)
			}

			behindTable := table.NewWriter()
			behindTable.SetOutputMirror(os.Stdout)
			behindTable.AppendHeader(table.Row{"Fork", "Behind by", "URL", "Stars", "Branches","Last updated"})
			for e := behind.Front(); e != nil; e = e.Next() {
				fork := e.Value.(Fork)
				behindBy := strconv.Itoa(fork.BehindBy)
				url := "https://github.com/" + string(fork.FullName)
				stars := strconv.Itoa(fork.Stars) + " \033[33m\u2605" + "\033[0m"
				branches := strconv.Itoa(fork.branchNumber)
				LastUpdated := dateHandler(fork.LastUpdated)
				behindTable.AppendRow([]interface{}{fork.FullName, behindBy, url, stars,branches, LastUpdated})
			}
			if *sortBy == "ahead" { // if sorting by ahead, the behind table needs to be sorted by behind by
				*sortBy = "behind"
				sortingHelper(behindTable, sortBy)
				*sortBy = "ahead"
			} else {
				sortingHelper(behindTable, sortBy)
			}
			if behind.Len() > 0 {
				platformPrint(color.Warn, fail+"Forks behind: "+strconv.Itoa(behind.Len()))
				behindTable.SetStyle(table.StyleRounded)
				behindTable.Render()
			} else {
				platformPrint(color.Notice, fail+"No forks behind of "+RepoInfo.Owner.Login+":"+*branch)
			}

			if *verboseFlag {
				evenTable := table.NewWriter()
				evenTable.AppendHeader(table.Row{"Fork", "URL", "Stars","Branches","Last updated"})
				for e := even.Front(); e != nil; e = e.Next() {
					fork := e.Value.(Fork)
					url := "https://github.com" + string(fork.FullName)
					stars := strconv.Itoa(fork.Stars) + " \033[33m\u2605" + "\033[0m"
					branches := strconv.Itoa(fork.branchNumber)
					fork.LastUpdated = dateHandler(fork.LastUpdated)
					evenTable.AppendRow([]interface{}{fork.FullName, url, stars,branches, fork.LastUpdated})
				}
				if *sortBy == "stars" || *sortBy == "lastUpdated" {
					sortingHelper(evenTable, sortBy)
				} else { // if sorting by ahead, the even table needs to be sorted by stars
					*sortBy = "stars"
					sortingHelper(evenTable, sortBy)
					*sortBy = "ahead"
				}
				if even.Len() > 0 {
					platformPrint(color.Notice, mitigate+"Forks up to date: "+strconv.Itoa(even.Len()))
					evenTable.SetStyle(table.StyleRounded)
					evenTable.Render()
				} else {
					platformPrint(color.Notice, fail+"No forks identical to "+RepoInfo.Owner.Login+":"+*branch)
				}

				deletedTable := table.NewWriter()
				deletedTable.SetOutputMirror(os.Stdout)
				deletedTable.AppendHeader(table.Row{"Fork", "URL","Stars","Branches","Last updated"})
				for e := deleted.Front(); e != nil; e = e.Next() {
					fork := e.Value.(Fork)
					url := "https://github.com" + string(fork.FullName)
					stars := strconv.Itoa(fork.Stars) + " \033[33m\u2605" + "\033[0m"
					branches := strconv.Itoa(fork.branchNumber)
					fork.LastUpdated = dateHandler(fork.LastUpdated)
					deletedTable.AppendRow([]interface{}{fork.FullName, url, stars, branches, fork.LastUpdated})
				}
				if deleted.Len() > 0 {
					platformPrint(color.Question, mitigate+"deleted forks: "+strconv.Itoa(deleted.Len()))
					deletedTable.SetStyle(table.StyleRounded)
					deletedTable.Render()
				} else {
					platformPrint(color.Notice, fail+"No deleted forks of "+RepoInfo.Owner.Login+":"+*branch)
				}
			}

			if ahead.Len() == 0 && behind.Len() == 0 && even.Len() == 0 && diverge.Len() == 0 && *branch == "master" {
				platformPrint(color.Error, fail+"No forks found on branch master maybe try with main?")
			}
		}
	}
}

func getConfigFilePath() string {
	//get the config file path depending on the OS
	var (
		ConfigFilePath string
		path           string
	)
	if runtime.GOOS == "windows" {
		ConfigFilePath, _ = os.UserConfigDir()
		ConfigFilePath += "\\gofork\\config.json"
	} else {
		path = os.Getenv("HOME") + "/.config/gofork/"
		ConfigFilePath = path + "gofork.conf"
	}
	return ConfigFilePath
}

func readConfig() string {
	var (
		auth Auth
	)
	//read the config file
	configFilePath := getConfigFilePath()
	dat, _ := os.ReadFile(configFilePath)
	json.Unmarshal([]byte(dat), &auth)
	return auth.Token
}

func writeConfig(token string) {
	//write the token to the config file depending on the OS
	cfp := getConfigFilePath()
	os.MkdirAll(cfp[:len(cfp)-11], 0777)
	ioutil.WriteFile(cfp, []byte(token), 0644)
	platformPrint(color.Success, "Token written to config file "+cfp)

}

func deleteConfig() error {
	configFilePath := getConfigFilePath()
	err := os.Remove(configFilePath)
	if err != nil {
		return err
	}
	return nil
}

func getInput() string {
	// get token from input and parses it with ParseInput()
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = parseInput(input)
	return input
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

func RepoCheck(repo string, token string) int {
	// checks if the repo is a valid github repo returns 0 if valid, 1 if not and 2 if there is an auth error. Any other error is returned as 3
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

func sortingHelper(mytable table.Writer, sortBy *string) table.Writer {
	// sorts the table depending on the sortBy variable
	if *sortBy == "stars" {
		mytable.SortBy([]table.SortBy{
			{Name: "Stars", Mode: table.DscNumeric},
		})
	} else if *sortBy == "lastUpdated" {
		mytable.SortBy([]table.SortBy{
			{Name: "Last Updated", Mode: table.Dsc},
		})
	} else if *sortBy == "ahead" {
		mytable.SortBy([]table.SortBy{
			{Name: "Ahead by", Mode: table.DscNumeric},
		})
	} else if *sortBy == "behind" { // should not be reachable except if the sorting mode is ahead and we have to sort behind table
		mytable.SortBy([]table.SortBy{
			{Name: "Behind by", Mode: table.AscNumeric},
		})
	} else if *sortBy == "branches" {
		mytable.SortBy([]table.SortBy{
			{Name: "Branches", Mode: table.DscNumeric},
		})
	}
	return mytable
}

func dateHandler(date string) string {
	// converts the date to a readable format
	date = strings.Replace(date, "T", " ", -1)
	date = strings.Replace(date, "Z", "", -1)
	return date
}
