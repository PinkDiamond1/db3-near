package db3

import (
    "blockwatch.cc/db3-near/pkg/near"
)

const (
    MAX_BLOCKS_TO_SETTLE = 120
    SECURITY_DEPOSIT     = 10000
    SLASHED_DEPOSIT_BIPS = 1000
)

type AccountID near.AccountID

type Host AccountID

type Database AccountID

type Pubkey near.Pubkey

type Signature near.Signature

type Money near.Money

type DBId uint64

type ApiEndpoint string

type CodeCID string

type QueryCID string

type ResultCID string

type SignedTransaction struct {
    Nonce    uint64
    Sender   AccountID
    Receiver AccountID
    Amount   int64
    CID      QueryCID
    Anchor   int64
    SignedBy Pubkey
    Sig      Signature
}

type SignedQuery struct {
    Db       Database
    Query    string
    CID      QueryCID
    Fee      SignedTransaction
    Sender   AccountID
    SignedBy Pubkey
    Sig      Signature
}

type SignedResult struct {
    QueryCID  QueryCID
    ResultCID ResultCID
    Result    string
    Host      Host
    SignedBy  Pubkey
    Sig       Signature
}

type Manifest struct {
    Author      AccountID
    Name        string
    License     string
    CID         CodeCID
    RoyaltyBips int
}

// Shared contract that manages all databases, deposits and payments
type ContractState struct {
    // contract owner (allowd to move slashed funds)
    Owner near.AccountID

    // registry
    NextId      DBId                    // id of the next deployed database (starts at 0)
    Owners      map[DBId]near.AccountID // royalty payments
    Manifests   map[DBId]Manifest       // discoverability of dbs for hosts/users
    ApiRegistry map[DBId]map[near.AccountID]ApiEndpoint

    // deposits
    Deposits map[DBId]map[near.AccountID]near.Money
    Slashed  near.Money

    // payment settlement
    ResultTTL        map[DBId]map[QueryCID]int64                        // latest block height
    PendingResults   map[DBId]map[QueryCID]map[near.AccountID]ResultCID // collected result hashes
    PendingFees      map[DBId]map[QueryCID]near.Money                   // fee proposed / paid
    SettledFees      map[near.AccountID]near.Money
    SettledRoyalties map[near.AccountID]near.Money
}

type Contract interface {
    // Registers a new database
    // Called by: developer
    Deploy(m Manifest) DBId

    // Locks security deposit when joining a new database
    // Called by: host
    Deposit(dbid DBId)

    // Unlocks and returns security deposit on leave
    // Called by: host
    Withdraw(dbid DBId)

    // Registers the host's API endpoint for a database
    // Called by: host
    Register(dbid DBId, uri ApiEndpoint)

    // Views all registered databases
    // Called by: user
    Databases() map[DBId]Manifest

    // Views all registered API endpoints for a database
    // Called by: user
    Discover(dbid DBId) []ApiEndpoint

    // Pays query fee
    // Called by: user (maybe injected by host)
    EscrowFee(dbid DBId, qid QueryCID, ttl int64)

    // Forwards fee payment tx and query execution proof
    Settle(dbid DBId, qid QueryCID, rid ResultCID)

    // Sends settled fees and royalties to claimer
    // Called by: host
    ClaimFees()

    // Sends settled royalties to claimer
    // Called by: developer
    ClaimRoyalties()

    // Recovers and transfers slashed funds
    // Called by: contract owner (DAO)
    Recover(amount near.Money, target near.AccountID)
}

type Node interface {
    // Executes a query (off-chain signed API call)
    Execute(query SignedQuery) SignedResult
}
