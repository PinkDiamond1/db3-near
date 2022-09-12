// Copyright (c) 2022 Blockwatch Data Inc.
// Author: alex@blockwatch.cc

package near

type AccountID string

type Pubkey string

type Signature string

type Money uint64

func (m Money) Mul(n int) Money {
    return m * Money(n)
}

func (m Money) Div(n int) Money {
    return m / Money(n)
}

type Signer interface {
    Sign([]byte) []byte
}

// Transaction context available during contract execution
type CallContext struct {
    Caller   AccountID // signer account id (near.predecessorAccountId)
    SignedBy Pubkey    // signer's pubkey
    Amount   Money     // sent amount (near.attachedDeposit)
    Height   int64     // near.blockIndex
}
