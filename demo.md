---
theme: https://raw.githubusercontent.com/charmbracelet/glamour/master/styles/dark.json
---

# Demo: Interacting with Boundary data in natural language 

---

# Intro
What is it?
- Langchain-based application leveraging LLM 
- OpenAI GPT-4 LLM

---

# Under the hood... 
Each question results in at least three requests to OpenAI's GPT-4 LLM:
1. Given the query and all the known tables in Boundary, which tables are most likely to be relevant to this query?
1. Given the query and the list of tables from the previous request, write a SQL query to get the relevant information.
1. Given the results of the SQL query and the user's query, craft a natural language response. 

---

# Error handling
In some cases, the SQL OpenAI sends back isn't perfect, and results in a SQL error. In that case, retry the prediction but include in that retry a new request to return a SQL query that fixes the error in the previously returned response. 

It will retry this SQL fix 10 times (you can set this with the `-max-retries` flag) before giving up.  

---

# Demo
- Running `boundary dev` and working against the local SQL database
- Inspect user hierarchy
    - How many users are there? 
    - How auth methods are there? 
    - What auth methods does the admin user have access to?
    - What user has access to the most scopes?
- Security incident response
    - how many sessions ocurred today?
    - What session egressed the most data?
    - How long did that session last?
    - What IP address and port did that session connect to?

---

# Caveats
There are a lot of these!
1. There is no AuthZ between this tool and Boundary's database: please use with care, as currently written, it's expected that the DSN you provide for postgres has access to all the tables in the database. 
1. There are no constraints on what SQL queries it will run: you can literally ask it "delete all targets" and it will run a DELETE on targets in the database, or any other valid query!
1. There has been no testing on accuracy.
1. This project is heavy on tokens, and makes no less than 3 requests to OpenAI per question. OpenAI charges by the token. Make sure to set that usage limit for your OpenAI account!
1. Currently the max number of tokens you can send to this service is 4k. Larger deployments means more data, and this project currently doesn't have a great way to cut the amount of relational data sent to OpenAI for analysis. If you have a larger deployment you will hit this limit more often than not. 
1. I have played around with creating vector embeddings of the relational data in Boundary and using a Vectorstore (Pinecone) in order to mitigate the data usage via similarity search. However, the vectors created from relational data were not very useful (I used ada-002 to make the embeddings). I'd like to experiment more with this approach in the future, but for now, tokens are a big limitation. 
1. There isn't any fine tuning: another to-do is to create a JSONL training set that is specific to the queries expected of Boundary users, and build a fine-tuned model. Right now your query mileage will vary. 


