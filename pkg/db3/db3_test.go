// Copyright (c) 2022 Blockwatch Data Inc.
// Author: alex@blockwatch.cc

package db3

import (
    "github.com/stretchr/testify/assert"
    "testing"

    "blockwatch.cc/db3-near/pkg/near"
)

func setCtx(caller, pk string, amount int64, height int64) {
    ctx = near.CallContext{
        Caller:   near.AccountID(caller),
        SignedBy: near.Pubkey(pk),
        Amount:   near.Money(amount),
        Height:   height,
    }
}

const (
    CALLER    = "sender.near"
    USER      = "user.near"
    NO_CALLER = "not_sender.near"
    PK        = "98793cd91a3f870fb126f66285808c7e094afcfc4eda8a970f6648cdf0dbd6de"
)

var (
    m1 = Manifest{
        Author:      "blockwatch.near",
        Name:        "Hello",
        License:     "n/a",
        CID:         "cid-1",
        RoyaltyBips: 1000,
    }
)

func TestDeploy(t *testing.T) {
    setCtx(CALLER, PK, 0, 10)
    db := NewDB3()
    id := db.Deploy(m1)
    assert.Equal(t, id, DBId(0), "first id")
    assert.Len(t, db.Manifests, 1, "manifest is stored")
    assert.NotNil(t, db.ApiRegistry[id], "registry map entry exists")
    assert.NotNil(t, db.Deposits[id], "deposits map entry exists")
    assert.NotNil(t, db.ResultTTL[id], "ttl map entry exists")
    assert.NotNil(t, db.PendingResults[id], "results map entry exists")
    assert.NotNil(t, db.PendingFees[id], "fees map entry exists")

    id = db.Deploy(Manifest{
        Name:        "Second without author",
        License:     "n/a",
        CID:         "cid-2",
        RoyaltyBips: 1000,
    })
    assert.Equal(t, id, DBId(1), "second id")
    assert.Len(t, db.Manifests, 2, "manifest is stored")
    assert.Equal(t, db.Manifests[id].Author, ctx.Caller, "replace empty manifest caller")
    assert.NotNil(t, db.ApiRegistry[id], "registry map entry exists")
    assert.NotNil(t, db.Deposits[id], "deposits map entry exists")
    assert.NotNil(t, db.ResultTTL[id], "ttl map entry exists")
    assert.NotNil(t, db.PendingResults[id], "results map entry exists")
    assert.NotNil(t, db.PendingFees[id], "fees map entry exists")

    assert.Panics(t, func() {
        db.Deploy(Manifest{
            Name:        "Negative royalty",
            Author:      "blockwatch.near",
            License:     "n/a",
            CID:         "cid-1",
            RoyaltyBips: -1,
        })
    }, "negative royalty")
    assert.Len(t, db.Manifests, 2, "manifest is not stored")

    assert.Panics(t, func() {
        db.Deploy(Manifest{
            Name:        "large royalty",
            Author:      "blockwatch.near",
            License:     "n/a",
            CID:         "cid-1",
            RoyaltyBips: 10001,
        })
    }, "royalty too large")
    assert.Len(t, db.Manifests, 2, "manifest is not stored")
}

func TestDepositSuccess(t *testing.T) {
    setCtx(CALLER, PK, SECURITY_DEPOSIT, 10)
    db := NewDB3()
    id := db.Deploy(m1)
    assert.NotPanics(t, func() { db.Deposit(id) }, "successful deposit")
    assert.Equal(t, db.Deposits[id][CALLER], near.Money(SECURITY_DEPOSIT), "correct deposit")
}

func TestDepositFail(t *testing.T) {
    setCtx(CALLER, PK, 1, 10)
    db := NewDB3()
    id := db.Deploy(m1)
    assert.Panics(t, func() { db.Deposit(id + 1) }, "no db")
    assert.Panics(t, func() { db.Deposit(id) }, "wrong deposit amount")
}

func TestWithdrawSuccess(t *testing.T) {
    setCtx(CALLER, PK, SECURITY_DEPOSIT, 10)
    db := NewDB3()
    id := db.Deploy(m1)
    db.Deposit(id)
    assert.NotPanics(t, func() { db.Withdraw(id) }, "successful withdraw")
    assert.Zero(t, db.Deposits[id][CALLER], "zero deposit")
}

func TestWithdrawFail(t *testing.T) {
    setCtx(CALLER, PK, 1, 10)
    db := NewDB3()
    id := db.Deploy(m1)
    assert.Panics(t, func() { db.Deposit(id + 1) }, "no db")
    setCtx(NO_CALLER, PK, 1, 10)
    assert.Panics(t, func() { db.Withdraw(id) }, "no deposit")
}

func TestRegisterSuccess(t *testing.T) {
    setCtx(CALLER, PK, SECURITY_DEPOSIT, 10)
    db := NewDB3()
    id := db.Deploy(m1)
    db.Deposit(id)
    assert.NotPanics(t, func() { db.Register(id, "myurl") }, "successful register")
    assert.Len(t, db.Discover(id), 1, "uri is discoverable")
    assert.ElementsMatch(t, db.Discover(id), []ApiEndpoint{"myurl"}, "correct uri")
    // overwrite
    assert.NotPanics(t, func() { db.Register(id, "anotherurl2") }, "successful re-register")
    assert.Len(t, db.Discover(id), 1, "uri is discoverable")
    assert.ElementsMatch(t, db.Discover(id), []ApiEndpoint{"anotherurl2"}, "correct uri")
    // unregister
    assert.NotPanics(t, func() { db.Register(id, "") }, "successful un-register")
    assert.Len(t, db.Discover(id), 0, "empty list")
}

func TestRegisterFail(t *testing.T) {
    setCtx(CALLER, PK, SECURITY_DEPOSIT, 10)
    db := NewDB3()
    id := db.Deploy(m1)
    db.Deposit(id)
    assert.Panics(t, func() { db.Register(id+1, "api") }, "no db")
    // simulate slash
    db.Deposits[id][ctx.Caller] /= 2
    assert.Panics(t, func() { db.Register(id, "api") }, "low deposit")
}

func TestFeeSuccess(t *testing.T) {
    setCtx(CALLER, PK, SECURITY_DEPOSIT, 10)
    db := NewDB3()
    id := db.Deploy(m1)
    db.Deposit(id)
    setCtx(USER, PK, 1, 10)
    assert.NotPanics(t, func() { db.EscrowFee(id, "cid-1", 10+MAX_BLOCKS_TO_SETTLE-1) }, "successful escrow")
    assert.Equal(t, db.PendingFees[id]["cid-1"], near.Money(1), "correct fee")
}

func TestFeeFail(t *testing.T) {
    setCtx(CALLER, PK, SECURITY_DEPOSIT, 10)
    db := NewDB3()
    id := db.Deploy(m1)
    db.Deposit(id)
    assert.Panics(t, func() { db.EscrowFee(id+1, "qid-1", 10+MAX_BLOCKS_TO_SETTLE) }, "no db")
    setCtx(CALLER, PK, SECURITY_DEPOSIT, 1000)
    assert.Panics(t, func() { db.EscrowFee(id, "qid-1", MAX_BLOCKS_TO_SETTLE) }, "expired")
}

// TODO:
// - Settle
// - Claim + finalize
// - Recover
