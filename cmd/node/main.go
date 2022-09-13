// Copyright (c) 2022 Blockwatch Data Inc.
// Author: alex@blockwatch.cc

package main

import (
    "context"
    // "encoding/base64"
    "encoding/json"
    "flag"
    "fmt"
    "math/big"
    "net/http"
    "os"
    "path/filepath"
    "time"

    "blockwatch.cc/near-api-go"
    // "blockwatch.cc/near-api-go/keys"
    // "blockwatch.cc/near-api-go/types"
    "github.com/echa/log"
    // "github.com/ethereum/go-ethereum/rpc"
    // "github.com/gorilla/schema"
    cid "github.com/ipfs/go-cid"
)

var (
    contractAddress string
    networkId       string
    accountId       string
    rpcEndpoint     string
    databaseId      string
    port            string
    // decoder         = schema.NewDecoder()
    conn    *near.Connection
    flags   = flag.NewFlagSet("node", flag.ContinueOnError)
    home    string
    account *near.Account
)

func init() {
    flags.Usage = func() {}
    flags.StringVar(&contractAddress, "contract", os.Getenv("DB3_CONTRACT_ID"), "DB3 contract")
    flags.StringVar(&accountId, "account", os.Getenv("DB3_NODE_ACCOUNT_ID"), "DB3 node account")
    flags.StringVar(&rpcEndpoint, "rpc", "https://rpc.testnet.near.org", "NEAR RPC endpoint")
    flags.StringVar(&networkId, "net", "testnet", "NEAR network id")
    flags.StringVar(&port, "port", "8000", "HTTP server port")

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

    if accountId == "" {
        return fmt.Errorf("Empty account id")
    }
    if contractAddress == "" {
        return fmt.Errorf("Empty contract id")
    }

    conn = near.NewConnection(rpcEndpoint)

    // Use a key pair directly as a signer.
    cfg := &near.Config{
        NetworkID: networkId,
        NodeURL:   rpcEndpoint,
        KeyPath:   filepath.Join(home, ".near-credentials", networkId, accountId+".json"),
    }
    account, err = near.LoadAccount(conn, cfg, accountId)
    if err != nil {
        return err
    }

    // use default http server
    log.Infof("Listening on :%s", port)
    http.HandleFunc("/", queryHandler)
    return http.ListenAndServe(":"+port, nil)
}

type SignedQuery struct {
    Db    string `json:"db"`
    Query string `json:"query"`
    Cid   string `json:"cid"`
    FeeTx []byte `json:"fee_tx"`
}

type SignedResult struct {
    QueryCID  string      `json:"query_cid"`
    ResultCID string      `json:"result_cid"`
    Result    interface{} `json:"result"`
    Sig       string      `json:"sig"`
}

func queryHandler(w http.ResponseWriter, r *http.Request) {
    // check method
    if r.Method != http.MethodPost {
        http.Error(w, "invalid method", http.StatusMethodNotAllowed)
        return
    }

    // parse query
    var query SignedQuery
    dec := json.NewDecoder(r.Body)
    err := dec.Decode(&query)
    if err != nil {
        log.Error(err)
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    r.Body.Close()

    c, err := cid.Decode(query.Cid)
    if err != nil {
        log.Error(err)
        http.Error(w, fmt.Sprintf("invalid cid: %v", err), http.StatusBadRequest)
        return
    }

    // TODO: check embedded transaction is valid and signed
    ctx := r.Context()

    // execute DB query
    result, err := executeQuery(ctx, query)
    if err != nil {
        log.Error(err)
        http.Error(w, fmt.Sprintf("query failed: %v", err), http.StatusInternalServerError)
        return
    }

    // create result CID
    buf, err := json.Marshal(result)
    if err != nil {
        log.Error(err)
        http.Error(w, fmt.Sprintf("marshal result: %v", err), http.StatusInternalServerError)
        return
    }
    c, err = c.Prefix().Sum(buf)
    if err != nil {
        log.Error(err)
        http.Error(w, fmt.Sprintf("encode cid: %v", err), http.StatusInternalServerError)
        return
    }

    // create result CID and return it to the user
    response := SignedResult{
        QueryCID:  query.Cid,
        ResultCID: c.String(),
        Result:    result,
        Sig:       "TODO",
    }
    buf, err = json.Marshal(response)
    if err != nil {
        log.Error(err)
        http.Error(w, fmt.Sprintf("marshal response: %v", err), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.Header().Set("Date", time.Now().Format(http.TimeFormat))
    w.WriteHeader(http.StatusOK)
    w.Write(buf)

    // sign settle call and broadcast embedded fee tx async
    go func() {
        // fee tx
        for retries := 3; retries > 0; retries-- {
            log.Infof("Broadcasting user tx")
            res, err := conn.SendTransactionAsync(query.FeeTx)
            if err == nil {
                log.Infof("Result: %#v", res)
                break
            } else {
                log.Error(err)
                <-time.After(time.Second)
            }
        }

        // sign and broadcast settle
        args, _ := json.Marshal(map[string]string{
            "dbid": query.Db,
            "qid":  query.Cid,
            "rid":  c.String(),
        })
        for retries := 3; retries > 0; retries-- {
            log.Infof("Sending settle call with args %s", string(args))
            res, err := account.FunctionCall(
                contractAddress,
                "settle",
                args,
                100_000_000_000_000,
                *big.NewInt(0),
            )
            if err == nil {
                log.Infof("Result: %#v", res)
                break
            } else {
                log.Error(err)
                <-time.After(time.Second)
            }
        }
    }()
}

func executeQuery(ctx context.Context, query SignedQuery) (interface{}, error) {
    log.Infof("Processing query db=%s cid=%s q=%q", query.Db, query.Cid, query.Query)
    return "Wsup?", nil
}
