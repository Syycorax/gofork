As github is now allows the sorting of forks, I'm now archiving this repo.

# Gofork

## Presentation

Gofork is a CLI tool to find forks that are ahead of a github repository.

## Usage

```
$ gofork --help
usage: gofork [-h|--help] -r|--repo "<value>" [-b|--branch "<value>"]
              [-v|--verbose] [-p|--page <integer>] [-s|--sort "<value>"]
              [-d|--deleteconfig]

              CLI tool to find active forks

Arguments:

  -h  --help          Print help information
  -r  --repo          Repository to check
  -b  --branch        Branch to check. Default: repo default branch
  -v  --verbose       Show deleted and up to date repositories
  -p  --page          Page to check (use -1 for all). Default: 1
  -s  --sort          Sort by (stars, ahead, lastUpdated). Default: ahead
  -d  --deleteconfig  Delete the config file
```

## Roadmap

* [x] Print the results in table
* [x] Add support for branches (with the default being the repo default branch)
* [x] Use terminal colors
* [x] Verbose flag for private/even forks
* [x] Loading bar
* [X] Flag to sort output
* [X] Add branches to output
* [ ] Flag to uninstall gofork
* [ ] Fix the sorting algorithm


## Built with

Built with love using [Golang](https://golang.org), [Github API](https://developer.github.com/v3/) and [akamensky's argparse](https://github.com/akamensky/argparse), [gookit's color](https://github.com/gookit/color), [jedib0t's go-pretty](github.com/jedib0t/go-pretty), [schollz's progressbar](https://github.com/schollz/progressbar) libraries.

## License

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)
