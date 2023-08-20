package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	_ "github.com/tmc/langchaingo/tools/sqldatabase/postgresql"
)

var (
	openAiAuthToken string
	psqlDsn         string
	maxRetries      int
)

type api struct {
	Paths map[string]struct {
		Path map[string]struct {
			Get map[string]struct {
				Summary    string `json:"summary"`
				Parameters []struct {
					Name        string `json:"name"`
					Description string `json:"description"`
					Required    string `json:"required"`
				} `json:"parameters"`
			} `json:"get"`
		}
	} `json:"paths"`
}

func main() {
	app := &cli.App{
		Name:  "boundary-ai",
		Usage: "Talk to your boundary postgres database using natural language",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "psql-dsn",
				Usage:       "Postgres database data source name (DSN), eg postgres://postgres:password@localhost:32768/boundary?sslmode=disable",
				EnvVars:     []string{"PSQL_DSN"},
				Destination: &psqlDsn,
			},
			&cli.StringFlag{
				Name:        "openai-api-key",
				Usage:       "OpenAI API key",
				EnvVars:     []string{"OPENAI_API_KEY"},
				Destination: &openAiAuthToken,
			},
			&cli.IntFlag{
				Name:        "max-retries",
				Value:       10,
				Usage:       "The maximum number of SQL retry attempts, defaults to 10.",
				Destination: &maxRetries,
			},
		},
		Action: func(*cli.Context) error {
			return run()
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	llm, err := openai.New()
	if err != nil {
		return err
	}

	fmt.Println("Conversation")
	fmt.Println("---------------------")
	fmt.Print("> ")
	s := bufio.NewScanner(os.Stdin)
	for s.Scan() {

		query := s.Text()

		paths, err := getRelevantPaths(query, llm)
		if err != nil {
			return err
		}

		tpl := fmt.Sprintf(`
		Given the following query and set of HTTP API paths, write syntatically correct HTTP request URLs to get the 
		relevant information using the base URL provided in the query. If no base URL is given, default to http://localhost:9200. 
		Query: %s
		Paths: %s`, query, paths)

		completion, err := llm.Call(context.Background(), tpl,
			llms.WithTemperature(0),
		)

		fmt.Println(completion)

		fmt.Print("> ")
	}
	return nil
}

func getBoundaryApiPaths() ([]string, error) {
	url := "https://raw.githubusercontent.com/hashicorp/boundary/main/internal/gen/controller.swagger.json"
	paths := []string{}

	resp, err := http.Get(url)
	if err != nil {
		return paths, err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return paths, err
	}

	jm := make(map[string]interface{})
	err = json.Unmarshal(body, &jm)
	if err != nil {
		return paths, err
	}

	for k, v := range jm["paths"].(map[string]interface{}) {
		fmt.Printf("Key: %s\nValue: %s\n\n", k, v)
		paths = append(paths, k)
	}

	return paths, nil
}

func getRelevantPaths(query string, llm llms.LLM) ([]string, error) {
	paths, err := getBoundaryApiPaths()
	if err != nil {
		return []string{}, err
	}

	tpl := fmt.Sprintf(`
Of the following HTTP API paths and the given query, return an ordered list of HTTP API requests that 
return relevant information about the query. 

Format the response to be a comma separated list.
Query: %s
API Paths %+v`, query, paths)

	ctx := context.Background()
	completion, err := llm.Call(ctx, tpl,
		llms.WithTemperature(0),
	)
	if err != nil {
		return []string{}, err
	}

	rawPaths := strings.Split(completion, ",")

	trimmedPaths := []string{}
	for _, p := range rawPaths {
		trimmedPaths = append(trimmedPaths, strings.TrimSpace(p))
	}
	paths = trimmedPaths

	return paths, nil
}

func retryPredict(ctx context.Context, c chains.Chain, input map[string]any, llm llms.LanguageModel, retries int, err error) (string, error) {
	tpl := fmt.Sprintf(`
Given the following query and error, return a syntatically correct SQL query that fixes the error.
Query: %s
Error: %s`, input["query"], err)

	input["query"] = tpl
	resp, err := chains.Predict(ctx, c, input)

	if err != nil {
		if retries == maxRetries {
			return "max retires reached, please try again, perhaps with a different phrasing?", err
		}
		retries++

		resp, err = retryPredict(ctx, c, input, llm, retries, err)
	}

	return resp, nil
}
