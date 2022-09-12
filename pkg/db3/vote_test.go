// Copyright (c) 2022 Blockwatch Data Inc.
// Author: alex@blockwatch.cc

package db3

import (
    "github.com/stretchr/testify/assert"
    "testing"
)

func TestUnanimousVote(t *testing.T) {
    e := NewElection()
    e.AddVote("A", "cid-1")
    e.AddVote("B", "cid-1")
    e.AddVote("C", "cid-1")
    assert.True(t, e.IsUnanimous(), "is unanimous")
    assert.True(t, e.IsSuperMajority(), "is super majority")
    assert.Equal(t, e.NumSuperMajority(), 3, "super majority count")
    assert.Equal(t, e.NumVoters(), 3, "voter count")
    assert.Len(t, e.SuperMajority(), 3, "super majority size matches all voters")
    assert.Len(t, e.Minority(), 0, "empty minority")
    assert.ElementsMatch(t, e.SuperMajority(), []Vote{
        {"A", "cid-1"},
        {"B", "cid-1"},
        {"C", "cid-1"},
    }, "super majority members match")
}

func TestMajority3(t *testing.T) {
    e := NewElection()
    e.AddVote("A", "cid-1")
    e.AddVote("B", "cid-1")
    e.AddVote("C", "cid-2")
    assert.False(t, e.IsUnanimous(), "is not unanimous")
    assert.True(t, e.IsSuperMajority(), "is super majority")
    assert.Equal(t, e.NumSuperMajority(), 2, "super majority count")
    assert.Equal(t, e.NumVoters(), 3, "voter count")
    assert.Len(t, e.SuperMajority(), 2, "super majority size matches two voters")
    assert.Len(t, e.Minority(), 1, "minority count")
    assert.ElementsMatch(t, e.SuperMajority(), []Vote{
        {"A", "cid-1"},
        {"B", "cid-1"},
    }, "super majority members match")
    assert.ElementsMatch(t, e.Minority(), []Vote{
        {"C", "cid-2"},
    }, "minority members match")
}

func TestFailedVote3(t *testing.T) {
    e := NewElection()
    e.AddVote("A", "cid-1")
    e.AddVote("B", "cid-2")
    e.AddVote("C", "cid-3")
    assert.False(t, e.IsUnanimous(), "is not unanimous")
    assert.False(t, e.IsSuperMajority(), "is super majority")
    assert.Equal(t, e.NumSuperMajority(), 0, "super majority count")
    assert.Equal(t, e.NumVoters(), 3, "voter count")
    assert.Len(t, e.SuperMajority(), 0, "super majority size is empty")
    assert.Len(t, e.Minority(), 3, "minority count")
    assert.ElementsMatch(t, e.Minority(), []Vote{
        {"A", "cid-1"},
        {"B", "cid-2"},
        {"C", "cid-3"},
    }, "minority members match")
}

func TestFailedVote2(t *testing.T) {
    e := NewElection()
    e.AddVote("A", "cid-1")
    e.AddVote("B", "cid-2")
    assert.False(t, e.IsUnanimous(), "is not unanimous")
    assert.False(t, e.IsSuperMajority(), "is super majority")
    assert.Equal(t, e.NumSuperMajority(), 0, "super majority count")
    assert.Equal(t, e.NumVoters(), 2, "voter count")
    assert.Len(t, e.SuperMajority(), 0, "super majority size is empty")
    assert.Len(t, e.Minority(), 2, "minority count")
    assert.ElementsMatch(t, e.Minority(), []Vote{
        {"A", "cid-1"},
        {"B", "cid-2"},
    }, "minority members match")
}

func TestFailedVote4(t *testing.T) {
    e := NewElection()
    e.AddVote("A", "cid-1")
    e.AddVote("B", "cid-2")
    e.AddVote("C", "cid-2")
    e.AddVote("D", "cid-3")
    assert.False(t, e.IsUnanimous(), "is not unanimous")
    assert.False(t, e.IsSuperMajority(), "is super majority")
    assert.Equal(t, e.NumSuperMajority(), 0, "super majority count")
    assert.Equal(t, e.NumVoters(), 4, "voter count")
    assert.Len(t, e.SuperMajority(), 0, "super majority size is empty")
    assert.Len(t, e.Minority(), 4, "minority count")
    assert.ElementsMatch(t, e.Minority(), []Vote{
        {"A", "cid-1"},
        {"B", "cid-2"},
        {"C", "cid-2"},
        {"D", "cid-3"},
    }, "minority members match")
}
