package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/tmc/langchaingo/chains"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/huggingface"
	"github.com/tmc/langchaingo/tools/sqldatabase"
	_ "github.com/tmc/langchaingo/tools/sqldatabase/postgresql"
)

var (
	hfApiToken string
	psqlDsn    string
	maxRetries int
)

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
				Name:        "hf-api-token",
				Usage:       "HuggingFace API key",
				EnvVars:     []string{"HUGGINGFACE_API_TOKEN"},
				Destination: &hfApiToken,
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
	llm, err := huggingface.New()
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()

	db, err := sqldatabase.NewSQLDatabaseWithDSN("pgx", psqlDsn, nil)
	if err != nil {
		return err
	}
	defer db.Close()

	sqlDatabaseChain := chains.NewSQLDatabaseChain(llm, 10, db)

	fmt.Println("Conversation")
	fmt.Println("---------------------")
	fmt.Print("> ")
	s := bufio.NewScanner(os.Stdin)
	for s.Scan() {

		tables, err := getRelevantTables(s.Text(), llm)
		if err != nil {
			return err
		}

		input := map[string]any{
			"query":              s.Text(),
			"table_names_to_use": tables,
		}

		out, err := chains.Predict(ctx, sqlDatabaseChain, input, chains.WithModel("bert-base-uncased"))
		if err != nil {
			out, err = retryPredict(ctx, sqlDatabaseChain, input, llm, 0, err)
		}
		fmt.Println(out)

		fmt.Print("> ")
	}
	return nil
}

func getRelevantTables(query string, llm llms.LLM) ([]string, error) {
	tables := []string{
		"auth_method",
		"auth_account",
		"auth_oidc_account",
		"auth_oidc_method",
		"auth_password_account",
		"auth_password_method",
		"auth_token",
		"host",
		"host_catalog",
		"host_dns_name",
		"host_ip_address",
		"host_set",
		"iam_group",
		"iam_group_member_user",
		"iam_group_role",
		"iam_role",
		"iam_role_grant",
		"iam_scope",
		"iam_scope_global",
		"iam_scope_org",
		"iam_scope_project",
		"iam_user",
		"iam_user_role",
		"session",
		"session_state",
		"session_connection",
		"session_target_address",
		"session_valid_state",
		"target",
	}

	tpl := fmt.Sprintf(`
Of the following list of tables and the given query, which tables are most relevant?
Format the response to be a comma separated list.
Query: %s
Tables %+v`, query, tables)

	ctx := context.Background()
	completion, err := llm.Call(ctx, tpl,
		llms.WithTemperature(0),
	)
	if err != nil {
		return []string{}, err
	}

	rawTables := strings.Split(completion, ",")

	trimmedTables := []string{}
	for _, t := range rawTables {
		trimmedTables = append(trimmedTables, strings.TrimSpace(t))
	}
	tables = trimmedTables

	return tables, nil
}

func retryPredict(ctx context.Context, c chains.Chain, input map[string]any, llm llms.LanguageModel, retries int, err error) (string, error) {
	tpl := fmt.Sprintf(`
Given the following query and error, return a syntatically correct SQL query that fixes the error.
Query: %s
Error: %s`, input["query"], err)

	input["query"] = tpl
	resp, err := chains.Predict(ctx, c, input, chains.WithModel("bert-base-uncased"))

	if err != nil {
		if retries == maxRetries {
			return "max retires reached, please try again, perhaps with a different phrasing?", err
		}
		retries++

		resp, err = retryPredict(ctx, c, input, llm, retries, err)
	}

	return resp, nil
}
