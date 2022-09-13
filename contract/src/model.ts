export const STORAGE_COST: bigint = BigInt("1000000000000000000000000") // 1 NEAR
export const SECURITY_DEPOSIT: bigint = BigInt("10000000000000000000000000000") // 10000 NEAR
export const SLASHED_DEPOSIT_BIPS: bigint = 2500n // 25% per offence
export const MAX_BLOCKS_TO_SETTLE: bigint = 120n // 120 blocks ~ 2min

export class Manifest {
  author_id: string;
  name: string;
  license: string;
  code_cid: string;
  royalty_bips: string;

  constructor({
    author_id,
    name,
    license,
    code_cid,
    royalty_bips,
  }:{
    author_id: string,
    name: string,
    license: string,
    code_cid: string,
    royalty_bips: string,
  }) {
    this.author_id = author_id;
    this.name = name;
    this.license = license;
    this.code_cid = code_cid;
    this.royalty_bips = royalty_bips;
  }
}