import { NearBindgen, near, call, view, initialize, LookupMap, UnorderedMap } from 'near-sdk-js';
import { assert, makekey, splitkey, scanmap } from './utils'
import { Manifest, STORAGE_COST, SECURITY_DEPOSIT, SLASHED_DEPOSIT_BIPS, MAX_BLOCKS_TO_SETTLE } from './model'
import { Election } from './vote'


@NearBindgen({})
class Db3Contract {
  owner: string = "db3.blockwatch.testnet";
  next_id: number = 0;
  db_owners: UnorderedMap = new UnorderedMap('map-dbid-owner');
  db_manifests: UnorderedMap = new UnorderedMap('map-dbid-manifest');
  db_api_registry: UnorderedMap = new UnorderedMap('map-dbid-api');
  db_deposits: LookupMap = new LookupMap('map-dbid-deposit');
  db_ttls: UnorderedMap = new UnorderedMap('map-dbid-ttl');
  db_pending_votes: UnorderedMap = new UnorderedMap('map-dbid-pending-results');
  db_pending_fees: LookupMap = new LookupMap('map-dbid-pending-fees');
  db_settled_fees: LookupMap = new LookupMap('map-dbid-settled-fees');
  db_settled_royalties: LookupMap = new LookupMap('map-dbid-settled-royalties');
  db_slashed: string = "0";

  @initialize({})
  init({ owner }:{owner: string}) {
    this.owner = owner
  }


  // Registers a new DB3 database and prepares storage
  @call({payableFunction: true})
  deploy({ manifest }: { manifest: Manifest }): string {
    let amount: bigint = near.attachedDeposit() as bigint;
    let royalty_bips = BigInt(manifest.royalty_bips || '0')
    assert(amount >= STORAGE_COST, `Attach at least ${STORAGE_COST} yoctoNEAR for storage`);
    assert(royalty_bips >= 0n && royalty_bips <= 10000n, "Royalty basis points out of range [0, 10000]")
    assert(manifest.code_cid.length > 0, "Empty code CID")
    let caller = near.signerAccountId()

    if (manifest.author_id.length === 0) {
      manifest.author_id = caller
    }

    let dbid:string = this.next_id.toString()
    this.db_owners.set(dbid, caller)
    this.db_manifests.set(dbid, manifest)

    this.next_id++
    return dbid;
  }

  // Locks security deposit when joining a new database or tops up slashed deposit
  @call({payableFunction: true})
  deposit({ dbid }: { dbid: string }): void {
    assert(parseInt(dbid) < this.next_id, "Database id does not exist")
    let caller = near.signerAccountId()
    let key = makekey(dbid, caller)
    let amount: bigint = near.attachedDeposit() as bigint;
    let newDeposit = BigInt(this.db_deposits.get(key) as string || '0') + amount
    assert(newDeposit >= SECURITY_DEPOSIT, "Security deposit too low")
    this.db_deposits.set(key, newDeposit.toString())
  }

  // Unlocks and returns security deposit on leave
  @call({})
  withdraw({ dbid }: { dbid: string }): void {
    assert(parseInt(dbid) < this.next_id, "Database id does not exist")
    let caller = near.signerAccountId()
    let key = makekey(dbid, caller)
    let toTransfer = BigInt(this.db_deposits.get(key) as string || '0')
    assert(toTransfer > 0n, "Caller did not pay deposit")

    // send the deposit back
    const promise = near.promiseBatchCreate(caller)
    near.promiseBatchActionTransfer(promise, toTransfer)

    // remove registrations, but keep in pending and settled maps
    this.db_deposits.remove(key)
    this.db_api_registry.remove(key)
  }

  // Registers the host's API endpoint for a database
  // TODO: require fee for storage accounting
  @call({})
  register_api({ dbid, uri }: { dbid: string, uri: string }): void {
    assert(parseInt(dbid) < this.next_id, "Database id does not exist")
    let caller = near.signerAccountId()
    let key = makekey(dbid, caller)
    let deposit = BigInt(this.db_deposits.get(key) as string || '0')
    assert(deposit >= SECURITY_DEPOSIT, "Security deposit too low")
    if (uri.length === 0) {
      this.db_api_registry.remove(key)
    } else {
      this.db_api_registry.set(key, uri)
    }
  }

  // Pays query fee
  @call({payableFunction: true})
  escrow({ dbid, qid, ttl }: { dbid: string, qid: string, ttl: number }): void {
    assert(parseInt(dbid) < this.next_id, "Database id does not exist")
    assert(ttl > near.blockIndex(), "TTL in the past")

    // add fees paid to current fees for this CID (multiple calls may run in parallel)
    let key = makekey(dbid, qid)
    let amount: bigint = near.attachedDeposit() as bigint;
    let newFee = BigInt(this.db_pending_fees.get(key) as string || '0') + amount
    this.db_pending_fees.set(key, newFee.toString())

    // store TTL unconditionally (this may override a TTL set via Settle,
    // but this case is expected)
    this.db_ttls.set(key, ttl)
  }

  // Settle stores a query execution proof
  @call({})
  settle({ dbid, qid, rid }: { dbid: string, qid: string, rid: string }): void {
    assert(parseInt(dbid) < this.next_id, "Database id does not exist")
    let caller = near.signerAccountId()
    let key = makekey(dbid, caller)
    let deposit = BigInt(this.db_deposits.get(key) as string || '0')
    assert(deposit >= SECURITY_DEPOSIT, "Security deposit too low")


    // check and init result TTL on first settlement (this should have been done
    // by calling EscrowFee, but we cannot assume this tx was published
    // or processed yet, this makes sure we can later garbage collect either way)
    let ttlkey = makekey(dbid, qid)
    let ttlval = this.db_ttls.get(ttlkey)
    let ttl: bigint
    if (!ttlval) {
      ttl = near.blockIndex() + MAX_BLOCKS_TO_SETTLE
      this.db_ttls.set(ttlkey, ttl.toString())
    } else {
      ttl = BigInt(ttlval as string);
    }
    let height = near.blockIndex()
    assert(ttl > height, `Settlement timed out ${ttlval} <= ${height}`)

    // allocate sub map when this is the first call for this query
    let votekey = makekey(dbid, qid, caller)
    this.db_pending_votes.set(votekey, rid)
  }

  @call({})
  claim(): void {
    // finalize all completed results, this amortizes gas costs across all
    // claimers, but note that the current implementation is not efficient
    this.internalFinalizeResults()

    let caller = near.signerAccountId()

    // sum fees and royalties
    let earnedFees = BigInt(this.db_settled_fees.get(caller) as string || '0')
    let earnedRoyalties = BigInt(this.db_settled_royalties.get(caller) as string || '0')
    let toTransfer = earnedFees + earnedRoyalties
    if (toTransfer > 0n) {
      const promise = near.promiseBatchCreate(caller)
      near.promiseBatchActionTransfer(promise, toTransfer)
    }
    this.db_settled_fees.remove(caller)
    this.db_settled_royalties.remove(caller)
  }

  @call({})
  finalize(): void {
    this.internalFinalizeResults()
  }

  // Recovers and transfers slashed funds
  @call({})
  recover({ amount, target }: { amount: string, target: string }): void {
    let caller = near.signerAccountId()
    assert(caller === this.owner, "Must be contract owner to recover funds")
    let slashedAmount = BigInt(this.db_slashed)
    let toTransfer = BigInt(amount)
    assert(slashedAmount <= toTransfer, "Amount is larger than available funds")
    if (toTransfer > 0n) {
      const promise = near.promiseBatchCreate(target)
      near.promiseBatchActionTransfer(promise, toTransfer)
      slashedAmount -= toTransfer
      this.db_slashed = slashedAmount.toString()
    }
  }

  // Views all registered databases
  @view({})
  databases(): Array<Manifest> {
    let dbs: Array<Manifest> = new Array()
    for ( let [_, v] of this.db_manifests) {
      dbs.push(v as Manifest)
    }
    return dbs
  }

  // Views own registered databases
  @view({})
  ownDatabases({owner}: {owner: string}): Array<Manifest> {
    let dbs: Array<Manifest> = new Array()
    for ( let [k, v] of this.db_owners) {
      if (owner !== v as string) {
        continue
      }
      let m = this.db_manifests.get(k as string)
      dbs.push(m as Manifest)
    }
    return dbs
  }

  @view({})
  manifest({ dbid }: { dbid: string }): Manifest {
    assert(parseInt(dbid) < this.next_id, "Database id does not exist")
    let m = this.db_manifests.get(dbid) as Manifest
    return m
  }

  // Views all registered API endpoints for a database
  @view({})
  discover({ dbid }: { dbid: string }): Array<string> {
    assert(parseInt(dbid) < this.next_id, "Database id does not exist")
    let uris: Array<string> = new Array()
    for (let [k, v] of this.db_api_registry) {
        let key = k as string
        if (!key.startsWith(dbid + "#")) {
          continue
        }
        uris.push(v as string)
    }
    return uris
  }

  // Views own earnings
  @view({})
  earned({owner}: {owner: string}): string {
    // sum settled caller fees and royalties
    let earnedFees = BigInt(this.db_settled_fees.get(owner) as string || '0')
    let earnedRoyalties = BigInt(this.db_settled_royalties.get(owner) as string || '0')
    let totalEarned = earnedFees + earnedRoyalties
    return totalEarned.toString()
  }

  internalSplitFeeOrSlash(
    { dbid,
      votes,
      feeToSplit,
      royalty_bips
    } : {
      dbid: string,
      votes: Map<string, string>,
      feeToSplit: bigint,
      royalty_bips: bigint
  }) {
    if (feeToSplit === 0n) {
      return
    }

    // pay developer royalty
    if (royalty_bips > 0) {
        let royaltyToPay = feeToSplit * royalty_bips / 10000n
        let owner = this.db_owners.get(dbid) as string
        let newRoyalty = BigInt(this.db_settled_royalties.get(owner) as string || '0')
        newRoyalty += royaltyToPay
        this.db_settled_royalties.set(owner, newRoyalty.toString())
        feeToSplit -= royaltyToPay
    }

    // check result votes, identify majority and slash offender
    //
    // SECURITY NOTE
    // this mechanism is very simple and prone to sybil attacks, so don't
    // use it in real life!
    //
    let election = new Election({votes})

    if (election.isSuperMajority()) {
      // case 1: all agree on the same result, no slashing, split payout
      // case 2: a >=2/3 supermajority exists -> slash all minority members
      let winners = election.superMajority()
      let feeShare = 10000n / BigInt(winners.length)
      let feeToShare = feeToSplit * feeShare / 10000n

      // share fee between all winners
      for (let vote of winners) {
          let newFee = BigInt(this.db_settled_fees.get(vote.account_id) as string || '0')
          newFee += feeToShare
          this.db_settled_fees.set(vote.account_id, newFee.toString())
          feeToSplit -= feeToShare
      }

      // send any dust to slashed
      let slashed = BigInt(this.db_slashed) + feeToSplit

      // slash minority
      let offenders = election.minority()
      if (offenders.length > 0) {
        for (let vote of offenders) {
            // calculate how much deposit to slash
            let key = makekey(dbid, vote.account_id)
            let deposit = BigInt(this.db_deposits.get(key) as string || '0')
            let amountToSlash = deposit * 10000n / SLASHED_DEPOSIT_BIPS

            // add to slashed
            slashed += amountToSlash

            // sub from deposit
            deposit -= amountToSlash
            this.db_deposits.set(key, deposit.toString())
        }
      }

      // store final slashed value
      this.db_slashed = slashed.toString()

    } else {
      // case 3: no supermajority exists -> send all fees to slashed pool
      // this case also applies when no result was published but the fee
      // payment was received for some reason
      let slashed = BigInt(this.db_slashed) + feeToSplit
      this.db_slashed = slashed.toString()
    }
  }

  internalFinalizeResults() {
    // - find all expired queries
    // - check voting results
    // - split fees
    // - slash offenders
    let height = near.blockIndex()

    let expired: Map<string, Map<string, string>> = new Map()

    // scan all databases for expired results
    for (let [k, v] of this.db_ttls) {
      let key = k as string
      let [dbid, qid] = splitkey(key)
      let ttl = v as number

      // skip results that did not yet reach their ttl
      if (ttl > height) {
        continue
      }
      let votes = scanmap(this.db_pending_votes, makekey(dbid, qid, ''))
      expired.set(key, votes)
    }

    for (let [k, v] of expired) {
      let [dbid, qid] = splitkey(k as string)
      let votes = v as Map<string, string>

      let manifest = this.db_manifests.get(dbid) as Manifest
      let royalty_bips = BigInt(manifest.royalty_bips)

      // fetch fee paid for this query; this assumes the fee payment transaction
      // was actually sent before TTL expired
      let fee = this.db_pending_fees.get(k)
      let feeToSplit = BigInt(fee as string || '0')

      // check result votes, pay fees and optionally slash offenders
      this.internalSplitFeeOrSlash({ dbid, votes, feeToSplit, royalty_bips })

      // clean up maps
      this.db_pending_fees.remove(k)
      this.db_ttls.remove(k)
      for ( [k] of votes ) {
        this.db_pending_votes.remove(k)
      }
    }
  }

}

