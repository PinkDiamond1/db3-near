export class Contract{
  wallet;

  constructor({wallet}){
    this.wallet = wallet;
  }

  // Views

  async databases(){
    return await this.wallet.viewMethod({method: 'databases'});
  }

  async ownDatabases(){
    return await this.wallet.viewMethod({method: 'ownDatabases', args:{ owner: this.wallet.accountId }});
  }

  async manifest({ dbid }){
    return await this.wallet.viewMethod({method: 'manifest', args:{ dbid }});
  }

  async discover({ dbid }){
    return await this.wallet.viewMethod({method: 'discover', args:{ dbid }});
  }

  async earned({ owner }){
    return await this.wallet.viewMethod({method: 'earned', args:{ owner }});
  }

  // Mutations

  async deploy( {
    author_id,
    name,
    license,
    code_cid,
    royalty_bips,
    deposit = "1000000000000000000000000"
  } ){
    return await this.wallet.callMethod({method: 'deploy', args:{manifest: {
      author_id,
      name,
      license,
      code_cid,
      royalty_bips,
    }}, deposit });
  }

  async deposit( { dbid, deposit = "10000000000000000000000000" } ){
    return await this.wallet.callMethod({method: 'deposit', args:{ dbid }, deposit });
  }

  async withdraw( { dbid } ){
    return await this.wallet.callMethod({method: 'withdraw', args:{ dbid }});
  }

  async register_api( { dbid, uri } ){
    return await this.wallet.callMethod({method: 'register_api', args:{ dbid, uri }});
  }

  async escrow( { dbid, deposit } ){
    return await this.wallet.callMethod({method: 'escrow', args:{ dbid }, deposit });
  }

  async claim( {} ){
    return await this.wallet.callMethod({method: 'claim', args:{} });
  }

  async finalize( {} ){
    return await this.wallet.callMethod({method: 'finalize', args:{} });
  }

  async recover( { amount, target} ){
    return await this.wallet.callMethod({method: 'recover', args:{ amount, target }});
  }


}