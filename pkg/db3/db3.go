package db3

import (
    "blockwatch.cc/db3-near/pkg/near"
)

type DB3 ContractState

func NewDB3() *DB3 {
    return &DB3{
        Owner:            "blockwatch.near",
        NextId:           0,
        Owners:           make(map[DBId]near.AccountID),
        Manifests:        make(map[DBId]Manifest),
        ApiRegistry:      make(map[DBId]map[near.AccountID]ApiEndpoint),
        Deposits:         make(map[DBId]map[near.AccountID]near.Money),
        Slashed:          0,
        ResultTTL:        make(map[DBId]map[QueryCID]int64),
        PendingResults:   make(map[DBId]map[QueryCID]map[near.AccountID]ResultCID),
        PendingFees:      make(map[DBId]map[QueryCID]near.Money),
        SettledFees:      make(map[near.AccountID]near.Money),
        SettledRoyalties: make(map[near.AccountID]near.Money),
    }
}

var (
    ctx    near.CallContext
    signer near.Signer
)

// Registers a new database
func (d *DB3) Deploy(m Manifest) DBId {
    dbid := d.NextId
    d.Owners[dbid] = ctx.Caller
    d.Manifests[dbid] = m

    // allocate accounting maps
    d.ApiRegistry[dbid] = make(map[near.AccountID]ApiEndpoint)
    d.Deposits[dbid] = make(map[near.AccountID]near.Money)
    d.ResultTTL[dbid] = make(map[QueryCID]int64)
    d.PendingResults[dbid] = make(map[QueryCID]map[near.AccountID]ResultCID)
    d.PendingFees[dbid] = make(map[QueryCID]near.Money)

    d.NextId++
    return dbid
}

// Locks security deposit when joining a new database or tops up slashed deposit
func (d *DB3) Deposit(dbid DBId) {
    if dbid >= d.NextId {
        panic("Database id does not exist")
    }
    deposit := d.Deposits[dbid][ctx.Caller]
    if deposit+ctx.Amount < SECURITY_DEPOSIT {
        panic("Security deposit too low")
    }
    d.Deposits[dbid][ctx.Caller] += ctx.Amount
}

// Unlocks and returns security deposit on leave
// Called by: host
func (d *DB3) Withdraw(dbid DBId) {
    if dbid >= d.NextId {
        panic("Database id does not exist")
    }
    deposit, ok := d.Deposits[dbid][ctx.Caller]
    if !ok {
        panic("Caller did not pay deposit")
    }
    delete(d.Deposits[dbid], ctx.Caller)
    near.TransferTo(ctx.Caller, deposit, signer)
}

// Registers the host's API endpoint for a database
// Called by: host
func (d *DB3) Register(dbid DBId, uri ApiEndpoint) {
    if dbid >= d.NextId {
        panic("Database id does not exist")
    }
    // ensure security deposit is paid
    deposit := d.Deposits[dbid][ctx.Caller]
    if deposit < SECURITY_DEPOSIT {
        panic("Security deposit too low")
    }
    if uri == "" {
        // remove
        delete(d.ApiRegistry[dbid], ctx.Caller)
    } else {
        // upsert
        d.ApiRegistry[dbid][ctx.Caller] = uri
    }
}

// Views all registered databases
// Called by: user
func (d *DB3) Databases() map[DBId]Manifest {
    return d.Manifests
}

// Views all registered API endpoints for a database
// Called by: user
func (d *DB3) Discover(dbid DBId) []ApiEndpoint {
    if dbid >= d.NextId {
        panic("Database id does not exist")
    }
    uris := make([]ApiEndpoint, 0, len(d.ApiRegistry[dbid]))
    for _, v := range d.ApiRegistry[dbid] {
        uris = append(uris, v)
    }
    return uris
}

// Pays query fee
// Called by: user (maybe injected by host)
func (d *DB3) EscrowFee(dbid DBId, qid QueryCID, ttl int64) {
    if dbid >= d.NextId {
        panic("Database id does not exist")
    }

    if ttl < ctx.Height {
        panic("Fee payment is expired")
    }

    // account fees paid
    d.PendingFees[dbid][qid] += ctx.Amount

    // store TTL unconditionally (this may override a TTL set via Settle,
    // but this case is expected)
    d.ResultTTL[dbid][qid] = ttl
}

// Forwards fee payment tx and query execution proof
func (d *DB3) Settle(dbid DBId, qid QueryCID, rid ResultCID) {
    if dbid >= d.NextId {
        panic("Database id does not exist")
    }

    // check security deposit is sufficient
    if d.Deposits[dbid][ctx.Caller] < SECURITY_DEPOSIT {
        panic("Security deposit too low")
    }

    // check and init result TTL on first settlement (this should have been done
    // by calling EscrowFee, but we cannot assume this tx was published
    // or processed yet, this makes sure we can later garbage collect either way)
    ttl, ok := d.ResultTTL[dbid][qid]
    if !ok {
        d.ResultTTL[dbid][qid] = ctx.Height + MAX_BLOCKS_TO_SETTLE
    } else if ttl <= ctx.Height {
        // TTL expired, we no longer accept results
        return
    }

    // allocate sub map when this is the first call for this query
    if _, ok = d.PendingResults[dbid][qid]; !ok {
        d.PendingResults[dbid][qid] = make(map[near.AccountID]ResultCID)
    }
    // store this host's result hash
    d.PendingResults[dbid][qid][ctx.Caller] = rid
}

// Sends settled fees and royalties to claimer
// Called by: host
func (d *DB3) ClaimFees() {
    // finalize all pending results
    d.finalizeResults()

    // check and return earned fees
    earned := d.SettledFees[ctx.Caller]
    if earned > 0 {
        near.TransferTo(ctx.Caller, earned, signer)
        d.SettledFees[ctx.Caller] -= earned
    }
}

// Sends settled royalties to claimer
// Called by: developer
func (d *DB3) ClaimRoyalties() {
    // finalize all pending results
    d.finalizeResults()

    // check and return earned fees
    earned := d.SettledRoyalties[ctx.Caller]
    if earned > 0 {
        near.TransferTo(ctx.Caller, earned, signer)
        d.SettledRoyalties[ctx.Caller] -= earned
    }
}

// Recovers and transfers slashed funds
// Called by: contract owner (DAO)
func (d *DB3) Recover(amount near.Money, target near.AccountID) {
    if ctx.Caller != d.Owner {
        panic("Must be contract owner to recover funds")
    }
    if d.Slashed < amount {
        panic("Amount is smaller than available funds")
    }
    d.Slashed -= amount
    near.TransferTo(target, amount, signer)
}

func (d *DB3) finalizeResults() {
    // for all expired queries, check result ids match and split fees and slash any offenders
    for dbid, ttls := range d.ResultTTL {
        royaltyBips := d.Manifests[dbid].RoyaltyBips
        for qid, ttl := range ttls {
            if ttl > ctx.Height {
                continue
            }

            // fetch fee paid for this query; this assumes the fee payment transaction
            // was actually sent before TTL expired
            if feeToSplit := d.PendingFees[dbid][qid]; feeToSplit > 0 {

                // pay developer royalty
                if royaltyBips > 0 {
                    royaltyToPay := feeToSplit.Mul(royaltyBips).Div(10000)
                    d.SettledRoyalties[d.Owners[dbid]] += royaltyToPay
                    feeToSplit -= royaltyToPay
                }

                // check results match, identify majority and slash offender
                //
                // SECURITY NOTE
                // this mechanism is very simple and prone to sybil attacks, so don't
                // use this in real life!
                //
                election := NewElection()
                for acc, rid := range d.PendingResults[dbid][qid] {
                    election.AddVote(acc, rid)
                }

                // check for majority
                switch {
                case election.IsUnanimous():
                    // case 1: all agree on the same result, no slashing, split payout
                    feeShare := 10000 / election.NumSuperMajority()
                    feeToShare := feeToSplit.Mul(feeShare).Div(10000)
                    for _, v := range election.SuperMajority() {
                        feeToSplit -= feeToShare
                        d.SettledFees[v.AccountId] += feeToShare
                    }
                    // send any dust to slashed
                    d.Slashed += feeToSplit

                case election.IsSuperMajority():
                    // case 2: a >=2/3 supermajority exists -> slash all minority members
                    feeShare := 10000 / election.NumSuperMajority()
                    feeToShare := feeToSplit.Mul(feeShare).Div(10000)
                    for _, v := range election.SuperMajority() {
                        feeToSplit -= feeToShare
                        d.SettledFees[v.AccountId] += feeToShare
                    }
                    // send any dust to slashed
                    d.Slashed += feeToSplit

                    // slash minority
                    for _, v := range election.Minority() {
                        amountToSlash := d.Deposits[dbid][v.AccountId] * 10000 / SLASHED_DEPOSIT_BIPS
                        d.Slashed += amountToSlash
                        d.Deposits[dbid][v.AccountId] -= amountToSlash
                    }

                default:
                    // case 3: no supermajority exists -> send all fees to slashed pool
                    // this case also applies when no result was published but the fee
                    // payment was received for some reason
                    d.Slashed += feeToSplit
                }

            }

            // clean up maps
            delete(d.PendingFees[dbid], qid)
            delete(d.PendingResults[dbid], qid)
            delete(d.ResultTTL[dbid], qid)
        }
    }
}
