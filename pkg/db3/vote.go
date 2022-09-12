// Copyright (c) 2022 Blockwatch Data Inc.
// Author: alex@blockwatch.cc

package db3

import (
    "blockwatch.cc/db3-near/pkg/near"
)

type Vote struct {
    AccountId near.AccountID
    ResultCID ResultCID
}

type Election struct {
    votes   []Vote
    results map[ResultCID]int
}

func NewElection() *Election {
    return &Election{
        votes:   make([]Vote, 0),
        results: make(map[ResultCID]int),
    }
}

func (e *Election) AddVote(voter near.AccountID, vote ResultCID) {
    e.votes = append(e.votes, Vote{
        AccountId: voter,
        ResultCID: vote,
    })
    e.results[vote]++
}

func (e Election) NumVoters() int {
    return len(e.votes)
}

func (e Election) NumSuperMajority() int {
    return len(e.SuperMajority())
}

func (e Election) IsUnanimous() bool {
    return len(e.results) == 1
}

func (e Election) IsSuperMajority() bool {
    numVoters := len(e.votes)
    cutoff := 200 * numVoters / 3
    for _, v := range e.results {
        if v*100 >= cutoff {
            return true
        }
    }
    return false
}

func (e Election) Minority() []Vote {
    minority := make([]Vote, 0)
    numVoters := len(e.votes)
    cutoff := 200 * numVoters / 3
    for _, vote := range e.votes {
        if e.results[vote.ResultCID]*100 < cutoff {
            minority = append(minority, vote)
        }
    }
    return minority
}

func (e Election) SuperMajority() []Vote {
    majority := make([]Vote, 0)
    numVoters := len(e.votes)
    cutoff := 200 * numVoters / 3
    for _, vote := range e.votes {
        if e.results[vote.ResultCID]*100 >= cutoff {
            majority = append(majority, vote)
        }
    }
    return majority
}
