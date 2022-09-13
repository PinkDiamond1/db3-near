# DB3 - Decentralized Database on NEAR

A [NEARCON](https://nearcon.org) 2022 Hackathon project by [Blockwatch Data Inc](https://blockwatch.cc)

## Intro

Decentralized networks heavily rely on centralized providers to host and access their data today. NEAR is no exception. For truly decentralized systems we need to think about how SaaS databases would look like in a web3 economy. How can you trust responses are correct, queries are not censored, and data is available at the latency your dapp requires?

DB3 is a decentralized web3 database network where service providers get paid for query execution. DB3 is permissionless, i.e. anyone can deploy, host, discover, and query databases. Developers implement database schemas + logic and deploy them as smart contracts. Hosts choose which databases to run based on interest and expected traffic. Hosts must lock a slashable security deposit and (co)-sign query results to prove correctness. Users attach fee payments to queries and send them directly to a set of hosts for execution. On successful and timely execution, the protocol pays out collected fees to hosts and a royalty share to developers.

## Design

<img width="750" alt="db3" src="https://user-images.githubusercontent.com/910436/189650945-b0d17ecb-ade7-4010-8fba-5e66a606df6c.png">

The DB3 on-chain protocol organizes fee payments and helps establish trust in query results. Fraudulent behaviour and censorship is discouraged by requiring hosts to post a security deposit that may be slashed if query results differ.

* **Developers** write database schema and ETL logic code and `publish` them on-chain using a custom DB3 smart contract (only the CID of the bundle is stored)
* **Hosts** choose which databases they are interested to host, then pay a security `deposit` and `register` their API endpoints on-chain; they utilize DB3 nodes to launch and sync databases; the node keeps databases in-sync by pulling and validating data from **trusted ingest sources** (out of scope for this hackathon)
* **Users** first `discover` API endpoints for databases they are interested in, then `sign` queries with attached **fee payments** and send them to selected hosts for execution
* After hosts have executed a query, they (1) `sign` the result, and (2) return it to the user immediately (to ensure low latency), and (3) `settle` fee and result with the database contract
* The database contract can split fees between hosts and developers who can `claim` payouts

Design choices for on-chain functions `deposit`, `register`, `settle`, and `claim`:

1. API registry, deposits, database content, and fee settlement can be organized into a single **combined** contract or **split** across multiple collaborating contracts.
2. On-chain database state can be **shared** (one set of contracts shared by all published databases) or **private** (one set of contracts that is private for each database).


## License

(c) 2022 - Blockwatch Data Inc - all rights reserved