import { NearBindgen, near, call, view, initialize, LookupMap, UnorderedMap } from 'near-sdk-js';
import { assert } from './utils'
import { Manifest, STORAGE_COST, SECURITY_DEPOSIT, SLASHED_DEPOSIT_BIPS, MAX_BLOCKS_TO_SETTLE } from './model'
import { Election } from './vote'


@NearBindgen({})
class DonationContract {
  owner: string = "db3.blockwatch.testnet";
  next_id: number = 0;
  db_owners: LookupMap = new LookupMap('map-dbid-owner');
  db_manifests: LookupMap = new LookupMap('map-dbid-manifest');
  db_api_registry: LookupMap = new LookupMap('map-dbid-api');
  db_deposits: LookupMap = new LookupMap('map-dbid-deposit');
  db_ttls: UnorderedMap = new UnorderedMap('map-dbid-ttl');
  db_pending_results: LookupMap = new LookupMap('map-dbid-pending-results');
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
    assert(amount > STORAGE_COST, `Attach at least ${STORAGE_COST} yoctoNEAR for storage`);
    assert(royalty_bips >= 0n && royalty_bips <= 10000n, "Royalty basis points out of range [0, 10000]")
    assert(manifest.code_cid.length > 0, "Empty code CID")

    if (manifest.author_id.length === 0) {
      manifest.author_id = near.signerAccountId()
    }

    let dbid:string = this.next_id.toString()
    this.db_owners.set(dbid, near.signerAccountId())
    this.db_manifests.set(dbid, manifest)
    this.db_api_registry.set(dbid, new UnorderedMap('map-accid-api'))
    this.db_deposits.set(dbid, new LookupMap('map-accid-deposit'))
    this.db_ttls.set(dbid, new LookupMap('map-cid-block'))
    this.db_pending_results.set(dbid, new UnorderedMap('map-cid-result'))
    this.db_pending_fees.set(dbid, new LookupMap('map-cid-fees'))

    this.next_id++
    return dbid;
  }

  // Locks security deposit when joining a new database or tops up slashed deposit
  @call({payableFunction: true})
  deposit({ dbid }: { dbid: string }): void {
    assert(parseInt(dbid) < this.next_id, "Database id does not exist")
    let caller = near.signerAccountId()
    let deposits = this.db_deposits.get(dbid) as LookupMap
    let amount: bigint = near.attachedDeposit() as bigint;
    let newDeposit = BigInt(deposits.get(caller) as string || '0') + amount
    assert(newDeposit >= SECURITY_DEPOSIT, "Security deposit too low")
    deposits.set(caller, newDeposit.toString())
  }

  // Unlocks and returns security deposit on leave
  @call({})
  withdraw({ dbid }: { dbid: string }): void {
    assert(parseInt(dbid) < this.next_id, "Database id does not exist")
    let caller = near.signerAccountId()
    let deposits = this.db_deposits.get(dbid) as LookupMap
    let toTransfer = BigInt(deposits.get(caller) as string || '0')
    assert(toTransfer > 0n, "Caller did not pay deposit")

    // send the deposit back
    const promise = near.promiseBatchCreate(caller)
    near.promiseBatchActionTransfer(promise, toTransfer)

    // remove registrations, but keep in pending and settled maps
    deposits.remove(caller)
    let registry = this.db_api_registry.get(dbid) as UnorderedMap
    registry.remove(caller)
  }

  // Registers the host's API endpoint for a database
  // TODO: require fee for storage accounting
  @call({})
  register_api({ dbid, uri }: { dbid: string, uri: string }): void {
    assert(parseInt(dbid) < this.next_id, "Database id does not exist")
    let caller = near.signerAccountId()
    let deposits = this.db_deposits.get(dbid) as LookupMap
    let deposit = BigInt(deposits.get(caller) as string || '0')
    assert(deposit >= SECURITY_DEPOSIT, "Security deposit too low")
    let registry = this.db_api_registry.get(dbid) as UnorderedMap
    if (uri.length === 0) {
      registry.remove(caller)
    } else {
      registry.set(caller, uri)
    }
  }

  // Views all registered databases
  @view({})
  databases() {
      return this.db_manifests
  }

  // Views all registered API endpoints for a database
  @view({})
  discover({ dbid }: { dbid: string }) {
    assert(parseInt(dbid) < this.next_id, "Database id does not exist")
    let uris: Array<string>
    for (let [k, v] of this.db_api_registry.get(dbid) as UnorderedMap) {
        uris.push(v as string)
    }
    return uris
  }

  // Pays query fee
  @call({payableFunction: true})
  escrow({ dbid, qid, ttl }: { dbid: string, qid: string, ttl: number }): void {
    assert(parseInt(dbid) < this.next_id, "Database id does not exist")
    assert(ttl < near.blockIndex(), "Fee payment is expired")

    // add fees paid to current fees for this CID (multiple calls may run in parallel)
    let amount: bigint = near.attachedDeposit() as bigint;
    let pending_fees = this.db_pending_fees.get(dbid) as LookupMap
    let newFee = BigInt(pending_fees.get(qid) as string || '0') + amount
    pending_fees.set(qid, newFee.toString())

    // store TTL unconditionally (this may override a TTL set via Settle,
    // but this case is expected)
    let ttls = this.db_ttls.get(dbid) as LookupMap
    ttls.set(qid, ttl)
  }

  // Settle stores a query execution proof
  @call({})
  settle({ dbid, qid, rid }: { dbid: string, qid: string, rid: string }): void {
    assert(parseInt(dbid) < this.next_id, "Database id does not exist")
    let caller = near.signerAccountId()
    let deposits = this.db_deposits.get(dbid) as LookupMap
    let deposit = BigInt(deposits.get(caller) as string || '0')
    assert(deposit >= SECURITY_DEPOSIT, "Security deposit too low")


    // check and init result TTL on first settlement (this should have been done
    // by calling EscrowFee, but we cannot assume this tx was published
    // or processed yet, this makes sure we can later garbage collect either way)
    let ttls = this.db_ttls.get(dbid) as LookupMap
    let ttl = BigInt(ttls.get(qid) as string || '0');
    if (!ttl) {
      ttl = near.blockIndex() + MAX_BLOCKS_TO_SETTLE
      ttls.set(qid, ttl.toString())
    }
    assert(ttl > near.blockIndex(), "Settlement timed out")

    // allocate sub map when this is the first call for this query
    let results = this.db_pending_results.get(dbid) as UnorderedMap
    let votes = results.get(qid) as UnorderedMap
    if (votes === null) {
      votes = new UnorderedMap("map-accid-votes")
      results.set(qid, votes)
    }
    votes.set(caller, rid)
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

  internalSplitFeeOrSlash(
    { dbid,
      votes,
      feeToSplit,
      royalty_bips
    } : {
      dbid: string,
      votes: UnorderedMap,
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
          this.db_settled_fees.set(vote.account_id, newFee)
          feeToSplit -= feeToShare
      }

      // send any dust to slashed
      let slashed = BigInt(this.db_slashed) + feeToSplit

      // slash minority
      let offenders = election.minority()
      if (offenders.length > 0) {
        let deposits = this.db_deposits.get(dbid) as LookupMap
        for (let vote of offenders) {
            // calculate how much deposit to slash
            let deposit = BigInt(deposits.get(vote.account_id) as string || '0')
            let amountToSlash = deposit * 10000n / SLASHED_DEPOSIT_BIPS

            // add to slashed
            slashed += amountToSlash

            // sub from deposit
            deposit -= amountToSlash
            deposits.set(vote.account_id, deposit.toString())
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

    // scan all databases for expired results
    for (let [k, v] of this.db_ttls) {
      let dbid = k as string
      let ttls = v as UnorderedMap
      let manifest = this.db_manifests.get(dbid) as Manifest
      let royalty_bips = BigInt(manifest.royalty_bips)

      // scan results for expiry
      for (let [k, ttl] of ttls) {
        let qid = k as string
        let ttl = v as number

        // skip results that did not yet reach their ttl
        if (ttl > height) {
          continue
        }

        // fetch fee paid for this query; this assumes the fee payment transaction
        // was actually sent before TTL expired
        let fees = this.db_pending_fees.get(dbid) as LookupMap
        let results = this.db_pending_results.get(dbid) as UnorderedMap
        let votes = results.get(qid) as UnorderedMap
        let feeToSplit = BigInt(fees.get(qid) as string || '0')

        // check result votes, pay fees and optionally slash offenders
        this.internalSplitFeeOrSlash({ dbid, votes, feeToSplit, royalty_bips })

        // clean up maps
        fees.remove(qid)
        results.remove(qid)
        ttls.remove(qid) // FIXME: check this is ok in this loop

      }
    }
  }

}

