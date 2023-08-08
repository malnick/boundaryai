# Boundary AI
Query your Boundary postgres database with natural language ([see caveats](#caveats))

## Example Usage
You can use any Boundary postgres database but these examples use [Boundary in dev mode](https://developer.hashicorp.com/boundary/docs/getting-started/dev-mode/dev-mode).

What users are there?
```
☁  boundaryai [main] ⚡  boundaryai
Conversation
---------------------
> how many users are there and what are their usernames?
There are 5 users and their usernames are anonymous, authenticated, recovery, user and admin.
> what username has access to the most scopes?
admin has access to the most scopes.
> what scopes does admin have access to?
Admin has access to global, o_1234567890, and p_1234567890.
```

What sessions were created?
```
☁  boundaryai [main] ⚡  boundaryai
Conversation
---------------------
> how many sessions were created today and what are their ids?
Today there is 1 session with id s_ITj77QQc8E.
> how much data was transfered in that session in megabytes?
The session transferred 479 megabytes of data.
> what ip address and port did that session establish a connection with?
The session established a connection with ip address 127.0.0.1 and port 50398.
> how long did that session last?
The session lasted 37 seconds.
```

## Under the Hood
Each question results in at least three requests to OpenAPI's GPT-4 LLM:
1. Given the query and all the known tables in Boundary, which tables are most likely to be relevant to this query?
1. Given the query and the list of tables from the previous request, write a SQL query to get the relevant information.
1. Given the results of the SQL query and the user's query, craft a natural language response. 

In some cases, the SQL OpenAI sends back isn't perfect, and results in a SQL error. In that case, retry the prediction but include in that retry a new request to return a SQL query that fixes the error in the previously returned response. 

It will retry this SQL fix 10 times (you can set this with the `-max-retries` flag) before giving up.  

## Caveats
There are plenty of caveats with this project, including but not limited to your own tolerance for non-deterministic outcomes. 

1. There is no AuthZ between this tool and Boundary's database: please use with care, as currently written, it's expected that the DSN you provide for postgres has access to all the tables in the database. 
1. There are no constraints on what SQL queries it will run: you can literally ask it "delete all targets" and it will run a DELETE on targets in the database, or any other valid query!
1. There has been no testing on accuracy.
1. This project is heavy on tokens, and makes no less than 3 requests to OpenAI per question. OpenAI charges by the token. Make sure to set that usage limit for your OpenAI account!
1. Currently the max number of tokens you can send to this service is 4k. Larger deployments means more data, and this project currently doesn't have a great way to cut the amount of relational data sent to OpenAI for analysis. If you have a larger deployment you will hit this limit more often than not. 
1. I have played around with creating vector embeddings of the relational data in Boundary and using a Vectorstore (Pinecone) in order to mitigate the data usage via similarity search. However, the vectors created from relational data were not very useful (I used ada-002 to make the embeddings). I'd like to experiment more with this approach in the future, but for now, tokens are a big limitation. 
1. There isn't any fine tuning: another to-do is to create a JSONL training set that is specific to the queries expected of Boundary users, and build a fine-tuned model. Right now your query mileage will vary. 


