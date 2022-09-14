import 'regenerator-runtime/runtime';
import { Contract } from './near-interface';
import { Wallet } from './near-wallet';
import { Hash } from 'ipfs-only-hash';

// create the Wallet and the Contract
const wallet = new Wallet({contractId: 'db6.echa.testnet'});
const contract = new Contract({wallet: wallet});

// Setup on page load
window.onload = async () => {
  let isSignedIn = await wallet.startUp();

  if(isSignedIn){
    signedInFlow();
  }else{
    signedOutFlow();
  }

  // fetchGreeting();
  getAndShowOwnDatabases();
  getAndShowAllDatabases();
  getAndShowAllDatabasesUser();
};

// Button clicks
document.querySelector('#sign-in-button').onclick = () => { wallet.signIn(); };
document.querySelector('#sign-out-button').onclick = () => { wallet.signOut(); }; 

// Take the new greeting and send it to the contract
async function doUserAction(event) {
  event.preventDefault();
  const { greeting } = event.target.elements;

  document.querySelector('#signed-in-flow main')
    .classList.add('please-wait');

  // await contract.setGreeting(greeting.value);

  // ===== Fetch the data from the blockchain =====
  // await fetchGreeting();
  document.querySelector('#signed-in-flow main')
    .classList.remove('please-wait');
}

// Get greeting from the contract on chain
// async function fetchGreeting() {
//   const currentGreeting = await contract.getGreeting();

//   document.querySelectorAll('[data-behavior=greeting]').forEach(el => {
//     el.innerText = currentGreeting;
//     el.value = currentGreeting;
//   });
// }

// Display the signed-out-flow container
function signedOutFlow() {
  document.querySelector('#signed-in-flow').style.display = 'none';
  document.querySelector('#signed-out-flow').style.display = 'block';
}

// Displaying the signed in flow container and fill in account-specific data
function signedInFlow() {
  document.querySelector('#signed-out-flow').style.display = 'none';
  document.querySelector('#signed-in-flow').style.display = 'block';
  document.querySelectorAll('[data-behavior=account-id]').forEach(el => {
    el.innerText = wallet.accountId;
  });
}

// On submit, get the greeting and send it to the contract
document.querySelector('#new-db-form').onsubmit = async (event) => {
  event.preventDefault()

  // get elements from the form using their id attribute
  const { fieldset, author_id, name, license, code_cid, royalty_bips } = event.target.elements

  // disable the form while the value gets updated on-chain
  fieldset.disabled = true

  try {
    await contract.deploy({
      author_id: author_id.value,
      name: name.value,
      license: license.value,
      code_cid: code_cid.value,
      royalty_bips: royalty_bips.value,
    })
  } catch (e) {
    alert(
      'Something went wrong! ' +
      'Maybe you need to sign out and back in? ' +
      'Check your browser console for more info.'
    )
    throw e
  }

  // re-enable the form, whether the call succeeded or failed
  fieldset.disabled = false
}

async function getAndShowOwnDatabases(){
  document.getElementById('database-table').innerHTML = 'Loading ...'

  let dbs = await contract.ownDatabases()

  document.getElementById('database-table').innerHTML = ''

  dbs.forEach(elem => {
    let tr = document.createElement('tr')
    tr.innerHTML = `
      <tr>
        <td> <a target="_new" href="https://explorer.testnet.near.org/?query=${elem.author_id}">${elem.author_id}</a> </td>
        <td>${elem.name}</td>
        <td>${elem.license}</td>
        <td><a target="_new" href="https://ipfs.io/ipfs/${elem.code_cid}">${shortHash(elem.code_cid)}</a></td>
        <td>${elem.royalty_bips/100}%</td>
        <td>
          <button class="button action">Claim Royalties</button>
        </td>
      </tr>
    `
    document.getElementById('database-table').appendChild(tr)
  })
}

function shortHash(h) {
  return h.slice(0,5) + "..." + h.slice(-5);
}

async function getAndShowAllDatabases(){
  document.getElementById('host-table').innerHTML = 'Loading ...'

  let dbs = await contract.databases()

  document.getElementById('host-table').innerHTML = ''

  dbs.forEach((elem,id) => {
    let tr = document.createElement('tr')
    tr.innerHTML = `
      <tr>
        <td> <a target="_new" href="https://explorer.testnet.near.org/?query=${elem.author_id}">${elem.author_id}</a> </td>
        <td>${elem.name}</td>
        <td>${elem.license}</td>
        <td><a target="_new" href="https://ipfs.io/ipfs/${elem.code_cid}">${shortHash(elem.code_cid)}</a></td>
        <td>${elem.royalty_bips/100}%</td>
        <td>
          <div class="flex">
            <button class="button action" id="host-btn-${id}" >Host</button>
            <button class="button action" id="link-btn-${id}">Link</button>
          </div>
        </td>
      </tr>
    `
    document.getElementById('host-table').appendChild(tr)
    document.getElementById('host-btn-'+id).onclick = () => { contract.deposit({dbid:id.toString()}); };
    document.getElementById('link-btn-'+id).onclick = () => {
      var uri = window.prompt("Add your API endpoint")
      contract.register_api({dbid:id.toString(), uri});
    };
  })
}

async function getAndShowAllDatabasesUser(){
  document.getElementById('user-table').innerHTML = 'Loading ...'

  let dbs = await contract.databases()

  document.getElementById('user-table').innerHTML = ''

  dbs.forEach((elem,id) => {
    let tr = document.createElement('tr')
    tr.innerHTML = `
      <tr>
        <td> <a target="_new" href="https://explorer.testnet.near.org/?query=${elem.author_id}">${elem.author_id}</a> </td>
        <td>${elem.name}</td>
        <td>${elem.license}</td>
        <td><a target="_new" href="https://ipfs.io/ipfs/${elem.code_cid}">${shortHash(elem.code_cid)}</a></td>
        <td>${elem.royalty_bips/100}%</td>
        <td>
          <button class="button action" id="query-btn-${id}">Query</button>
        </td>
      </tr>
    `
    document.getElementById('user-table').appendChild(tr)
    document.getElementById('query-btn-'+id).onclick = async () => {
      var query = window.prompt("Enter your DB query");

      // fetch database endpoints
      var uris = await contract.discover({dbid: id.toString()});
      console.log("discovered", uris)

      // calculate content id
      const data = Buffer.from(query);
      const cid = await Hash.of(data);
      console.log("cid", cid)

      // get latest block and add TTL to height
      var res = await wallet.getLatestBlock();
      var ttl = (res + 120).toString();
      console.log("latest block", res)

      // send paymeng tx
      await contract.escrow({dbid:id.toString(), qid, ttl});

      // run query against one of the uris
      // TODO

    };
  })
}