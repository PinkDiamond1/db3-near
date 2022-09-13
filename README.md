# DB3 - Decentralized Database on NEAR

A [NEARCON](https://nearcon.org) 2022 Hackathon project by [Blockwatch Data Inc](https://blockwatch.cc)

## Intro

Decentralized networks heavily rely on centralized providers to host and access their data today. NEAR is no exception. For truly decentralized systems we need to think about how SaaS databases would look like in a web3 economy. How can you trust responses are correct, queries are not censored, and data is available at the latency your dapp requires?

DB3 is a decentralized web3 database network where service providers get paid for query execution. DB3 is permissionless, i.e. anyone can deploy, host, discover, and query databases. Developers implement database schemas + logic and deploy them as smart contracts. Hosts choose which databases to run based on interest and expected traffic. Hosts must lock a slashable security deposit and (co)-sign query results to prove correctness. Users attach fee payments to queries and send them directly to a set of hosts for execution. On successful and timely execution, the protocol pays out collected fees to hosts and a royalty share to developers.

## Design

<img width="750" alt="db3" src="https://user-images.githubusercontent.com/910436/189650945-b0d17ecb-ade7-4010-8fba-5e66a606df6c.png">

The DB3 on-chain protocol organizes fee payments and helps establish trust in query results. Fraudulent behaviour and censorship is discouraged by requiring hosts to post a security deposit that may be slashed if query results differ.

* **Developers** write database schema and ETL logic code and `publish` them on-chain using a custom DB3 smart contract (only the CID of the bundle is stored)
* **Hosts** choose which databases they are interested to host, then pay a security `deposit` and `register` their API endpoints on-chain. Hosts utilize DB3 nodes to launch and sync databases. The node keeps databases in sync by pulling and validating data from **trusted ingest sources** (this is out of scope for this hackathon)
* **Users** first `discover` API endpoints for databases they are interested in, then `sign` queries with attached **fee payments** and send them to selected hosts for execution
* After hosts have executed a query, they (1) `sign` the result, (2) return it to the user immediately (to ensure low latency), and (3) `settle` the fee and result with the database contract
* The database contract can split fees between hosts and developers who can `claim` payouts

Design choices for on-chain functions `deposit`, `register`, `settle`, and `claim`:

1. API registry, deposits, database content, and fee settlement can be organized in a single **combined** smart contract or **split** across multiple collaborating contracts.
2. On-chain database state can be **shared** (one set of contracts shared by all published databases) or **private** (one set of contracts that is private for each database).


## How to run

Create a testnet wallet on https://wallet.testnet.near.org/ and install the NEAR CLI tools via `npm install -g near-cli`. Replace the NEAR testnet account below with your own.

```sh
# login to your testnet account
near login

# create funded accounts for DB3 database contract and one or multiple nodes
near create-account node1.echa.testnet --masterAccount echa.testnet --initialBalance 10
near create-account db3.echa.testnet --masterAccount echa.testnet --initialBalance 10

# build and deploy the contract (we initialize it right away by setting the owner)
cd contract
npm run build
near deploy --accountId db6.echa.testnet --wasmFile build/db3_near.wasm --initFunction init --initArgs '{"owner": "echa.testnet"}'

# deploy a new database (this tx also pays for storage allocation, so add some Near)
# the call returns the db id ("0" for the first)
near call db3.echa.testnet deploy '{ "manifest": { "author_id": "echa.testnet", "name": "Hello NEAR", "license": "all rights reserved", "code_cid": "QmehH9PrVpiXKXS6upTS8uBoYecGaQvBXyw351tGQVQg2c", "royalty_bips": "1000"}}' --accountId echa.testnet --amount 1

# list databases
near view db3.echa.testnet databases
near view db3.echa.testnet ownDatabases '{"owner":"echa.testnet"}'

# pay deposit for your node
near call db3.echa.testnet deposit '{"dbid":"0"}' --accountId node1.echa.testnet --amount 10

# register your API endpoint
near call db3.echa.testnet register_api '{"dbid":"0","uri":"http://localhost:8000"}' --accountId node1.echa.testnet

# Now its time to send queries - this is a 3-step process, first the user (query sender)
# creates a fee payment, then the database node settles a result hash, finally the call
# closes after TTL expiry and the fee is paid to db owner and node
#
# Note that TTL is a block height, so you must first read the most recent network block
# height and add a small delay, we use 120 blocks (2 min) as default

# send fee payment for a query identified by CID and set TTL
near call db3.echa.testnet escrow '{"dbid":"0","qid":"query-1","ttl":100112999}' --amount 1 --accountId echa.testnet

# settle result hash for the query CID
near call db3.echa.testnet settle '{"dbid":"0","qid":"query-1","rid":"result-1"}' --accountId node1.echa.testnet

# we can manually call finalize (this also happens during claim, but for demo purposes we will see that fees are paid out after TTL expires)
near call db3.echa.testnet finalize --accountId echa.testnet

# now we can check how much everyone has earned
near view db3.echa.testnet earned '{"owner":"node1.echa.testnet"}'
near view db3.echa.testnet earned '{"owner":"echa.testnet"}'

# ... and claim earned fees
near call db3.echa.testnet claim --accountId node1.echa.testnet
near call db3.echa.testnet claim --accountId echa.testnet
```

The query and settlement steps can also be performed with the two Go programs `node` (a DB3 database node) and `sim` (a query client).

```sh
# run the node, it exposes its query API on localhost:8000; the node waits for queries,
# executes them and then forwards fee payment and settles the result hash
go run ./cmd/node/ -contract db3.echa.testnet -account node1.echa.testnet

# run the client which will send a mock query with fee payment to the node
go run ./cmd/sim/ -contract db3.echa.testnet -query 'SELECT * FROM hello_near' -account echa.testnet
````


## License

(c) 2022 - Blockwatch Data Inc - all rights reserved