package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
)

func main() {
	if len(os.Args) != 4 {
		printUsage()
		os.Exit(1)
	}

	url := os.Args[1]
	token := os.Args[2]
	dir := os.Args[3]

	if !path.IsAbs(dir) {
		cwd, err := os.Getwd()
		if err != nil {
			panic(err.Error())
		}
		dir = path.Join(cwd, dir)
	}

	fi, err := os.Stat(dir)
	if err != nil {
		panic(err.Error())
	}

	if !fi.IsDir() {
		fmt.Println("ERROR: supplied path not a directory")
	}

	list := buildList(dir, []string{})
	postToMemos(url, token, list)
}

type memo struct {
	title string
	body  string
	path  string
	tags  []string
}

func (m *memo) String() string {
	return fmt.Sprintf(`===
title: %s
path: %s
tags: %s

`, m.title, m.path, strings.Join(m.tags, ", "))
}

var markdownFileRegex = regexp.MustCompile(`^(.+)\.[Mm][dD]$`)

func buildList(dir string, tags []string) []memo {
	fi, err := os.Stat(dir)
	if err != nil {
		panic(err.Error())
	}
	if !fi.IsDir() {
		return []memo{}
	}

	list, err := os.ReadDir(dir)
	if err != nil {
		panic(err.Error())
	}

	var memos []memo
	for _, f := range list {
		absPath := path.Join(dir, f.Name())
		if f.IsDir() {
			tag := strings.ReplaceAll(strings.ToLower(f.Name()), " ", "-")
			newTags := append(tags, tag)
			memos = append(memos, buildList(absPath, newTags)...)
		} else {
			matches := markdownFileRegex.FindStringSubmatch(f.Name())
			if len(matches) != 2 {
				continue
			}

			body, err := os.ReadFile(absPath)
			if err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
			}

			memos = append(memos, memo{
				title: matches[1],
				path:  absPath,
				body:  string(body),
				tags:  tags,
			})
		}
	}

	return memos
}

type memoRequest struct {
	Content    string `json:"content"`
	Visibility string `json:"visibility"`
}

func postToMemos(baseURL, token string, memos []memo) error {
	apiURL, err := url.JoinPath(baseURL, "/api/v1/memos")
	if err != nil {
		return err
	}

	client := http.Client{}
	for _, m := range memos {
		body, err := json.Marshal(memoRequest{
			Content:    fmt.Sprintf("# %s\n%s\n%s", m.title, m.body, tagsString(m.tags)),
			Visibility: "PRIVATE",
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			continue
		}

		req, err := http.NewRequest("POST", apiURL, bytes.NewReader(body))
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			continue
		}

		req.Header.Add("Authorization", "Bearer "+token)
		req.Header.Add("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			continue
		}

		switch resp.StatusCode {
		case http.StatusOK, http.StatusCreated:
			continue

		default:
			fmt.Fprintf(os.Stderr, "unexpected status code: %d\n", resp.StatusCode)
		}
	}

	return nil
}

func tagsString(tags []string) string {
	sb := strings.Builder{}

	for _, t := range tags {
		sb.WriteString("#" + t + " ")
	}

	return strings.TrimSpace(sb.String())
}

func printUsage() {
	fmt.Println("USAGE: obsidian2memos <URL> <TOKEN> <DIR>")
	fmt.Println("\tURL\tThe URL of the memos instance; when specifying an IP address remember to include the port.")
	fmt.Println("\tTOKEN\tThe memos API token.")
	fmt.Println("\tDIR\tDirectory containing any number of markdown files and subdirectories.")
}
