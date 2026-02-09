package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Graph [][]string

var colors = [][]int{
	{15, 23, 30},   // dark grey
	{0, 64, 24},    // dark green
	{0, 100, 32},   // medium green
	{0, 140, 40},   // bright green
	{80, 220, 100}, // neon green
}

func main() {
	if len(os.Args) < 2 {
		reset := "\033[0m"
		fmt.Println(`
░▒█▀▀▄░▒█▀▀█░▒█▀▀▄░█▀▀▄░▒█▀▀█░▒█░▒█
░▒█░░░░▒█░▄▄░▒█▄▄▀▒█▄▄█░▒█▄▄█░▒█▀▀█
░▒█▄▄▀░▒█▄▄▀░▒█░▒█▒█░▒█░▒█░░░░▒█░▒█
		` + reset)

		fmt.Println("usage: cgraph <command> [args...]")
		return
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	configDir, err := os.UserConfigDir()
	if err != nil {
		panic(err)
	}

	appDir := filepath.Join(configDir, "cgraph")
	err = os.MkdirAll(appDir, 0755)
	if err != nil {
		panic(err)
	}

	path := filepath.Join(appDir, "account.txt")

	var accountName string

	switch cmd {
	case "change":
		if exists(path) {
			os.Remove(path)
		}
		cmd = "checkme"
	case "check":
		if len(args) > 0 {
			accountName = args[0]
		} else {
			fmt.Println("usage: cgraph check <github username>")
			os.Exit(0)
		}
	case "checkme":
		if exists(path) {
			data, err := os.ReadFile(path)
			if err != nil {
				panic(err)
			}
			accountName = string(data)
		} else {
			fmt.Print("github username> ")
			fmt.Scan(&accountName)

			err := os.WriteFile(path, []byte(accountName), 0644)
			if err != nil {
				panic(err)
			}
		}
	default:
		fmt.Println("wrong command")
	}

	body, err := fetchContributionsPage(accountName)
	if err != nil {
		log.Fatal(err)
	}

	formattedDate := getCurrentDate()

	contributionsBody, totalContributions, mostContributions, avgContributions, sameDateCount := getContributions(body, formattedDate)

	// get the todaysContributions from iterating contributionsBody once (it's more efficient)
	var todaysContributions string
	r, c := 0, 0

	skipFirstSameDateContribution := true
	changeColLimit := false
	colLimit := 53
	var graph Graph
	for _, contribution := range contributionsBody {
		if len(graph) <= r {
			graph = append(graph, make([]string, 0, 53))
		}

		contributionNumStr := fetchContributionNum(contribution)
		num, _ := strconv.Atoi(contributionNumStr)
		var element string

		switch {
		case num == 0:
			element = block(colors[0][0], colors[0][1], colors[0][2]) // level 0

		case num < avgContributions:
			element = block(colors[1][0], colors[1][1], colors[1][2]) // level 1

		case num < mostContributions/2:
			element = block(colors[2][0], colors[2][1], colors[2][2]) // level 2

		case num < mostContributions:
			element = block(colors[3][0], colors[3][1], colors[3][2]) // level 3

		default:
			element = block(colors[4][0], colors[4][1], colors[4][2]) // level 4
		}

		if strings.Contains(contribution, formattedDate) {
			if sameDateCount > 1 && skipFirstSameDateContribution {
				skipFirstSameDateContribution = false
			} else {
				todaysContributions = fetchContributionNum(contribution)
				changeColLimit = true
			}
		} 
		graph[r] = append(graph[r], element)

		c++
		if c == colLimit {
			if changeColLimit {
				changeColLimit = false
				colLimit = 52
			}
			c = 0
			r++
		}
	}

	printGraph(graph, totalContributions, avgContributions, todaysContributions, accountName)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	exists := err == nil

	return exists
}

func fetchContributionsPage(accountName string) ([]byte, error) {
	client := &http.Client{}
	contributionsUrl := "https://github.com/users/" + accountName + "/contributions"
	req, _ := http.NewRequest("GET", contributionsUrl, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")	
	
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)

	return body, err
}

func getCurrentDate() string {
	currentMonth := time.Now().Month().String()
	currentTime := time.Now()

	exampleFormat := currentMonth + " 2"
	formattedDate := currentTime.Format(exampleFormat)

	return formattedDate
}

func getContributions(body []byte, formattedDate string) ([]string, int, int, int, int) {
	re := regexp.MustCompile(`(?s)<tool-tip.*?>.*?</tool-tip>`)
	sections := re.FindAll(body, -1)

	re1 := regexp.MustCompile(`(?s)<h2.*?>.*?</h2>`)
	match := re1.Find(body)

	var activeDays int
	var totalContributions int
	if match != nil {
		re := regexp.MustCompile(`(?s)</?h2[^>]*>`)
		clean := re.ReplaceAllString(string(match), "")
		clean = strings.TrimSpace(clean)

		numRe := regexp.MustCompile(`([\d,]+)`)
		numStr := numRe.Find([]byte(clean))

		num, _ := strconv.Atoi(strings.Replace(string(numStr), ",", "", -1))
		totalContributions = num
	}

	re = regexp.MustCompile(`>(.*?)</tool-tip>`)

	var contributions []string
	var sameDateCount int
	mostContributions := 0
	for _, section := range sections {
		match := re.FindStringSubmatch(string(section))
		if len(match) > 1 {
			contribution := match[1]
			// hacky way
			if strings.Contains(contribution, formattedDate) {
				sameDateCount++
			}
			contributionNumStr := fetchContributionNum(contribution)
			num, _ := strconv.Atoi(contributionNumStr)
			if num > mostContributions {
				mostContributions = num
			}
			contributionNum := num
			if contributionNum >= 1 {
				activeDays++
			}

			contributions = append(contributions, contribution)
		}
	}

	var avgContributions int
	if activeDays == 0 {
		avgContributions = 0
	} else {
		avgContributions = (totalContributions / activeDays)
	}

	return contributions, totalContributions, mostContributions, avgContributions, sameDateCount
}

func fetchContributionNum(contribution string) string {
	contributionNum := strings.Split(contribution, " ")[0]

	return contributionNum
}

func block(r, g, b int) string {
    return fmt.Sprintf("\033[48;2;%d;%d;%dm  \033[0m", r, g, b)
}

func printGraph(graph Graph, totalContributions, avgContributions int, todaysContributions, accountName string) {
	graphStr := ""
	for _, row := range graph {
		for _, col := range row {
			graphStr += col
		}
		graphStr += "\n"
	}

	fmt.Println(graphStr)
	fmt.Println("username: " + accountName + "\n")
	fmt.Println("(in one year)")
	fmt.Println("total contributions: " + strconv.Itoa(totalContributions))
	fmt.Println("avg. contributions: " +  strconv.Itoa(avgContributions))
	if todaysContributions != "" {
		fmt.Println("\ncontributions today: " + todaysContributions)
	} else {
		fmt.Println("\nNo contributions today")
	}
}