// Copyright (c) 2022 Blockwatch Data Inc.
// Author: alex@blockwatch.cc

package main

import (
    "bytes"
    "encoding/json"
    "flag"
    "fmt"
    "math/big"
    "net/http"
    "os"
    "path/filepath"
    "strconv"

    "blockwatch.cc/near-api-go"
    "github.com/echa/log"
    cid "github.com/ipfs/go-cid"
    mc "github.com/multiformats/go-multicodec"
    mh "github.com/multiformats/go-multihash"
    "github.com/near/borsh-go"
)

var (
    contractAddress string
    networkId       string
    accountId       string
    rpcEndpoint     string
    nodeEndpoint    string
    databaseId      string
    queryString     string
    ttl             int64
    feeString       string
    flags           = flag.NewFlagSet("sim", flag.ContinueOnError)
    home            string
)

func init() {
    flags.Usage = func() {}
    flags.StringVar(&contractAddress, "contract", os.Getenv("NEAR_CONTRACT_ID"), "DB3 contract")
    flags.StringVar(&accountId, "account", os.Getenv("NEAR_ACCOUNT_ID"), "signer account")
    flags.StringVar(&rpcEndpoint, "rpc", "https://rpc.testnet.near.org", "NEAR RPC endpoint")
    flags.StringVar(&nodeEndpoint, "node", "http://localhost:8000", "DB3 node RPC endpoint")
    flags.StringVar(&databaseId, "db", "0", "DB3 database id")
    flags.StringVar(&networkId, "net", "testnet", "NEAR network id")
    flags.StringVar(&queryString, "query", "", "query string")
    flags.StringVar(&feeString, "fee", "1000000000000000000000000", "query fee in yoctoNear (1 Near = 10^24)")
    flags.Int64Var(&ttl, "ttl", 120, "TX TTL in blocks")

    var err error
    home, err = os.UserHomeDir()
    if err != nil {
        panic(err)
    }
}

func main() {
    if err := run(); err != nil {
        log.Fatalf("Error: %v\n", err)
    }
}

type Query struct {
    Db    string `json:"db"`
    Query string `json:"query"`
}

type SignedQuery struct {
    Query
    Cid   string `json:"cid"`
    FeeTx []byte `json:"fee_tx"`
}

type SignedResult struct {
    QueryCID  string      `json:"query_cid"`
    ResultCID string      `json:"result_cid"`
    Result    interface{} `json:"result"`
    Sig       string      `json:"sig"`
}

func run() error {
    err := flags.Parse(os.Args[1:])
    if err != nil {
        if err == flag.ErrHelp {
            fmt.Printf("Usage: %s [flags]\n", os.Args[0])
            fmt.Println("\nFlags")
            flags.PrintDefaults()
            return nil
        }
        return err
    }

    if databaseId == "" {
        return fmt.Errorf("Empty database id")
    }
    if accountId == "" {
        return fmt.Errorf("Empty account id")
    }

    conn := near.NewConnection(rpcEndpoint)

    // Use a key pair directly as a signer.
    cfg := &near.Config{
        NetworkID: networkId,
        NodeURL:   rpcEndpoint,
        KeyPath:   filepath.Join(home, ".near-credentials", networkId, accountId+".json"),
    }
    account, err := near.LoadAccount(conn, cfg, accountId)
    if err != nil {
        return err
    }

    // assemble query data
    q := Query{
        Db:    databaseId,
        Query: queryString,
    }
    buf, err := json.Marshal(q)
    if err != nil {
        return err
    }

    // create query content hash
    pref := cid.Prefix{
        Version:  1,
        Codec:    uint64(mc.Raw),
        MhType:   mh.SHA2_256,
        MhLength: -1, // default length
    }
    c, err := pref.Sum(buf)
    if err != nil {
        return err
    }
    log.Infof("Using query cid %s", c.String())

    fee := new(big.Int)
    fee.SetString(feeString, 10)

    // fetch current block height
    stat, err := conn.GetNodeStatus()
    if err != nil {
        return err
    }
    height, _ := stat["sync_info"].(map[string]interface{})["latest_block_height"].(json.Number).Int64()
    log.Infof("NEAR %s is on block %d", networkId, height)

    // create near transaction
    args, _ := json.Marshal(map[string]string{
        "dbid": databaseId,
        "qid":  c.String(),
        "ttl":  strconv.FormatInt(ttl+height, 10),
    })

    // account := account.NewAccount(config, accountId)
    _, signedTx, err := account.SignTransaction(contractAddress, []near.Action{{
        Enum: 2,
        FunctionCall: near.FunctionCall{
            MethodName: "escrow",
            Args:       args,
            Gas:        100_000_000_000_000,
            Deposit:    *fee,
        },
    }})
    if err != nil {
        return err
    }

    // serialize transaction
    buf, err = borsh.Serialize(signedTx)
    if err != nil {
        return fmt.Errorf("serializing signed transaction: %v", err)
    }

    // prepare and send database query
    squery := SignedQuery{
        Query: q,
        Cid:   c.String(),
        FeeTx: buf,
    }
    buf, _ = json.Marshal(squery)
    log.Infof("Signed query %s", string(buf))

    // call database node
    resp, err := http.Post(nodeEndpoint, "application/json", bytes.NewBuffer(buf))
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    var res SignedResult
    dec := json.NewDecoder(resp.Body)
    err = dec.Decode(&res)
    if err != nil {
        return err
    }
    log.Infof("Signed result %#v", res)

    return nil
}
