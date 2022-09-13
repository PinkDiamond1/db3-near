import { UnorderedMap } from 'near-sdk-js';

export class Vote {
  account_id: string;
  result_cid: string;

  constructor({ account_id, result_cid}: { account_id: string, result_cid: string}) {
    this.account_id = account_id;
    this.result_cid = result_cid;
  }
}

export class Election {
  votes: Array<Vote>;
  results: Map<string, number>;

  constructor({ votes }:{ votes: UnorderedMap }) {
    for (let [ k, v ] of votes) {
      let account_id = k as string
      let result_cid = v as string
      this.votes.push(new Vote({ account_id, result_cid }))
      let count = this.results.get(result_cid) || 0
      this.results.set(result_cid, count + 1)
    }
  }

  numVoters(): number {
    return this.votes.length
  }

  numSuperMajority(): number {
    return this.superMajority().length
  }

  isUnanimous(): boolean {
    return this.results.size === 1
  }

  isSuperMajority(): boolean {
    let numVoters = this.votes.length
    let cutoff = 200 * numVoters / 3
    for (let [ _, v ] of this.results) {
        if (v*100 >= cutoff) {
            return true
        }
    }
    return false
  }

  minority(): Array<Vote> {
    let minority: Array<Vote>
    let numVoters = this.votes.length
    let cutoff = 200 * numVoters / 3
    for (let vote of this.votes) {
      if (this.results.get(vote.result_cid)*100 < cutoff) {
          minority.push(vote)
      }
    }
    return minority
  }

  superMajority(): Array<Vote> {
    let majority: Array<Vote>
    let numVoters = this.votes.length
    let cutoff = 200 * numVoters / 3
    for (let vote of this.votes) {
      if (this.results.get(vote.result_cid)*100 >= cutoff) {
          majority.push(vote)
      }
    }
    return majority
  }
}