import { UnorderedMap } from 'near-sdk-js';

export function assert(statement, message) {
  if (!statement) {
    throw Error(`Assertion failed: ${message}`)
  }
}

export function makekey(...args: string[]) {
  return args.join("#")
}

export function splitkey(key: string): Array<string> {
  return key.split("#")
}

export function scanmap(m: UnorderedMap, prefix: string): Map<string,string> {
  let res: Map<string,string> = new Map()
  for (let [k, v] of m) {
    let key = k as string
    if (!key.startsWith(prefix)) {
      continue
    }
    res.set(key, v as string)
  }
  return res
}