<p align="center">
  <img width="90%" alt="tokenvm" src="assets/logo.png">
</p>
<p align="center">
  Mint, Transfer, and Trade User-Generated Tokens, All On-Chain
</p>

---

We created the [`tokenvm`](./examples/tokenvm) to showcase how to use the
`hypersdk` in an application most readers are already familiar with, token minting
and token trading. The `tokenvm` lets anyone create any asset, mint more of
their asset, modify the metadata of their asset (if they reveal some info), and
burn their asset. Additionally, there is an embedded on-chain exchange that
allows anyone to create orders and fill (partial) orders of anyone else. To
make this example easy to play with, the `tokenvm` also bundles a powerful CLI
tool and serves RPC requests for trades out of an in-memory order book it
maintains by syncing blocks.

If you are interested in the intersection of exchanges and blockchains, it is
definitely worth a read (the logic for filling orders is < 100 lines of code!).

## Status
`tokenvm` is considered **ALPHA** software and is not safe to use in
production. The framework is under active development and may change
significantly over the coming months as its modules are optimized and
audited.

## Features
### Arbitrary Token Minting
The basis of the `tokenvm` is the ability to create, mint, and transfer user-generated
tokens with ease. When creating an asset, the owner is given "admin control" of
the asset functions and can later mint more of an asset, update its metadata
(during a reveal for example), or transfer/revoke ownership (if rotating their
key or turning over to their community).

Assets are a native feature of the `tokenvm` and the storage engine is
optimized specifically to support their efficient usage (each balance entry
requires only 72 bytes of state = `assetID|publicKey=>balance(uint64)`). This
storage format makes it possible to parallelize the execution of any transfers
that don't touch the same accounts. This parallelism will take effect as soon
as it is re-added upstream by the `hypersdk` (no action required in the
`tokenvm`).

### Trade Any 2 Tokens
What good are custom assets if you can't do anything with them? To showcase the
raw power of the `hypersdk`, the `tokenvm` also provides support for fully
on-chain trading. Anyone can create an "offer" with a rate/token they are
willing to accept and anyone else can fill that "offer" if they find it
interesting. The `tokenvm` also maintains an in-memory order book to serve over
RPC for clients looking to interact with these orders.

Orders are a native feature of the `tokenvm` and the storage engine is
optimized specifically to support their efficient usage (just like balances
above). Each order requires only 152 bytes of
state = `orderID=>inAsset|inTick|outAsset|outTick|remaining|owner`. This
storage format also makes it possible to parallelize the execution of any fills
that don't touch the same order (there may be hundreds or thousands of orders
for the same pair, so this still allows parallelization within a single pair
unlike a pool-based trading mechanism like an AMM). This parallelism will take
effect as soon as it is re-added upstream by the `hypersdk` (no action required
in the `tokenvm`).

#### In-Memory Order Book
To make it easier for clients to interact with the `tokenvm`, it comes bundled
with an in-memory order book that will listen for orders submitted on-chain for
any specified list of pairs (or all if you prefer). Behind the scenes, this
uses the `hypersdk's` support for feeding accepted transactions to any
`hypervm` (where the `tokenvm`, in this case, uses the data to keep its
in-memory record of order state up to date). The implementation of this is
a simple max heap per pair where we arrange best on the best "rate" for a given
asset (in/out).

#### Sandwich-Resistant
Because any fill must explicitly specify an order (it is up to the client/CLI to
implement a trading agent to perform a trade that may span multiple orders) to
interact with, it is not possible for a bot to jump ahead of a transaction to
negatively impact the price of your execution (all trades with an order occur
at the same price). The worst they can do is to reduce the amount of tokens you
may be able to trade with the order (as they may consume some of the remaining
supply).

Not allowing the chain or block producer to have any control over what orders
a transaction may fill is a core design decision of the `tokenvm` and is a big
part of what makes its trading support so interesting/useful in a world where
producers are willing to manipulate transactions for their gain.

#### Partial Fills and Fill Refunds
Anyone filling an order does not need to fill an entire order. Likewise, if you
attempt to "overfill" an order the `tokenvm` will refund you any extra input
that you did not use. This is CRITICAL to get right in a blockchain-context
because someone may interact with an order just before you attempt to acquire
any remaining tokens...it would not be acceptable for all the assets you
pledged for the fill that weren't used to disappear.

#### Expiring Fills
Because of the format of `hypersdk` transactions, you can scope your fills to
be valid only until a particular time. This enables you to go for orders as you
see fit at the time and not have to worry about your fill sitting around until you
explicitly cancel it/replace it.

## Demos
Someone: "Seems cool but I need to see it to really get it."
Me: "Look no further."

The first step to running these demos is to launch your own `tokenvm` Subnet. You
can do so by running the following command from this location (may take a few
minutes):
```bash
./scripts/run.sh;
```

_By default, this allocates all funds on the network to
`token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp`. The private
key for this address is
`0x323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7`.
For convenience, this key has is also stored at `demo.pk`._

To make it easy to interact with the `tokenvm`, we implemented the `token-cli`.
Next, you'll need to build this. You can use the following command from this location
to do so:
```bash
./scripts/build.sh
```

_This command will put the compiled CLI in `./build/token-cli`._

Lastly, you'll need to add the chains you created and the default key to the
`token-cli`. You can use the following commands from this location to do so:
```bash
./build/token-cli key import demo.pk
./build/token-cli chain import-anr
```

_`chain import-anr` connects to the Avalanche Network Runner server running in
the background and pulls the URIs of all nodes tracking each chain you
created._

### Mint and Trade
#### Step 1: Create Your Asset
First up, let's create our own asset. You can do so by running the following
command from this location:
```bash
./build/token-cli action create-asset
```

When you are done, the output should look something like this:
```
database: .token-cli
address: token1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdj73w34s
chainID: 2btpGNEt7pGXxYUMk7Pp3TTiq8ij4JsrDxTv9oNB46wpwqLVQ9
symbol: MARIO
decimals: 2
metadata: its a me, mario
continue (y/n): y
✅ txID: o7bJT3aRwgLR19c2avPSKBNjdrCsPgxBXA3oc1MeLvvEKKNP7
assetID: 2r9rd3oUGfir4pnmkyy7aBUcScQrkuqbYjdmxaJDAe1pb2Je8u
```

The "loaded address" here is the address of the default private key (`demo.pk`). We
use this key to authenticate all interactions with the `tokenvm`.

#### Step 2: Mint Your Asset
After we've created our own asset, we can now mint some of it. You can do so by
running the following command from this location:
```bash
./build/token-cli action mint-asset
```

When you are done, the output should look something like this (usually easiest
just to mint to yourself).
```
database: .token-cli
address: token1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdj73w34s
chainID: 2btpGNEt7pGXxYUMk7Pp3TTiq8ij4JsrDxTv9oNB46wpwqLVQ9
assetID: 2r9rd3oUGfir4pnmkyy7aBUcScQrkuqbYjdmxaJDAe1pb2Je8u
symbol: MARIO decimals: 2 metadata: its a me, mario supply: 0
amount: 100
continue (y/n): y
✅ txID: Zrq8CUXeXQqjX97QnNAd4RRi2SiWz14cHyMh16NjQ15vFakPg
```

#### Step 3: View Your Balance
Now, let's check that the mint worked right by checking our balance. You can do
so by running the following command from this location:
```bash
./build/token-cli key balance
```

When you are done, the output should look something like this:
```
database: .token-cli
address: token1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdj73w34s
chainID: 2btpGNEt7pGXxYUMk7Pp3TTiq8ij4JsrDxTv9oNB46wpwqLVQ9
assetID (use TKN for native token): 2r9rd3oUGfir4pnmkyy7aBUcScQrkuqbYjdmxaJDAe1pb2Je8u
uri: http://127.0.0.1:9652/ext/bc/2btpGNEt7pGXxYUMk7Pp3TTiq8ij4JsrDxTv9oNB46wpwqLVQ9
symbol: MARIO decimals: 2 metadata: its a me, mario supply: 10000
balance: 100.00 MARIO
```

#### Step 4: Create an Order
So, we have some of our token (`MARIO`)...now what? Let's put an order
on-chain that will allow someone to trade the native token (`TKN`) for some.
You can do so by running the following command from this location:
```bash
./build/token-cli action create-order
```

When you are done, the output should look something like this:
```
database: .token-cli
address: token1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdj73w34s
chainID: 2btpGNEt7pGXxYUMk7Pp3TTiq8ij4JsrDxTv9oNB46wpwqLVQ9
in assetID (use TKN for native token): TKN
in tick: 1
out assetID (use TKN for native token): 2r9rd3oUGfir4pnmkyy7aBUcScQrkuqbYjdmxaJDAe1pb2Je8u
symbol: MARIO decimals: 2 metadata: its a me, mario supply: 10000
balance: 100.00 MARIO
out tick: 10
supply (must be multiple of out tick): 100
continue (y/n): y
✅ txID: 23xyvTKHZUy9ym2VC8ysdsaRz9xbLyfuUoRgGXn6zmotuvxw2e
orderID: 2hJdJV2JisA1ESQ2SbGapjHn9ZetouU6TyptMNLsRgufQK1hVc
```

The "in tick" is how much of the "in assetID" that someone must trade to get
"out tick" of the "out assetID". Any fill of this order must send a multiple of
"in tick" to be considered valid (this avoids ANY sort of precision issues with
computing decimal rates on-chain).

#### Step 5: Fill Part of the Order
Now that we have an order on-chain, let's fill it! You can do so by running the
following command from this location:
```bash
./build/token-cli action fill-order
```

When you are done, the output should look something like this:
```
database: .token-cli
address: token1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdj73w34s
chainID: 2btpGNEt7pGXxYUMk7Pp3TTiq8ij4JsrDxTv9oNB46wpwqLVQ9
in assetID (use TKN for native token): TKN
balance: 9999999999.999856949 TKN
out assetID (use TKN for native token): 2r9rd3oUGfir4pnmkyy7aBUcScQrkuqbYjdmxaJDAe1pb2Je8u
symbol: MARIO decimals: 2 metadata: its a me, mario supply: 10000
available orders: 1
0) Rate(in/out): 1000000.0000 InTick: 1.000000000 TKN OutTick: 10.00 MARIO Remaining: 100.00 MARIO
select order: 0 [auto-selected]
value (must be multiple of in tick): 2
in: 2.000000000 TKN out: 20.00 MARIO
continue (y/n): y
✅ txID: B2MCLPTHouQMzd2dfxod2qiJC8p58vSahSDGVhZEJCCvQpzXB
```

Note how all available orders for this pair are listed by the CLI (these come
from the in-memory order book maintained by the `tokenvm`).

#### Step 6: Close Order
Let's say we now changed our mind and no longer want to allow others to fill
our order. You can cancel it by running the following command from this
location:
```bash
./build/token-cli action close-order
```

When you are done, the output should look something like this:
```
database: .token-cli
address: token1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdj73w34s
orderID: 2hJdJV2JisA1ESQ2SbGapjHn9ZetouU6TyptMNLsRgufQK1hVc
out assetID (use TKN for native token): 2r9rd3oUGfir4pnmkyy7aBUcScQrkuqbYjdmxaJDAe1pb2Je8u
continue (y/n): y
✅ txID: KuPqXinaS2H7uDLh6LriPCMyn8DzNQP9XZ8Lv7wWjY88sDK4A
```

Any funds that were locked up in the order will be returned to the creator's
account.

#### Bonus: Watch Activity in Real-Time
To provide a better sense of what is actually happening on-chain, the
`token-cli` comes bundled with a simple explorer that logs all blocks/txs that
occur on-chain. You can run this utility by running the following command from
this location:
```bash
./build/token-cli chain watch
```

If you run it correctly, you'll see the following input (will run until the
network shuts down or you exit):
```
database: .token-cli
available chains: 2 excluded: []
0) chainID: Em2pZtHr7rDCzii43an2bBi1M2mTFyLN33QP1Xfjy7BcWtaH9
1) chainID: cKVefMmNPSKmLoshR15Fzxmx52Y5yUSPqWiJsNFUg1WgNQVMX
select chainID: 0
watching for new blocks on Em2pZtHr7rDCzii43an2bBi1M2mTFyLN33QP1Xfjy7BcWtaH9 👀
height:13 txs:1 units:488 root:2po1n8rqdpNuwpMGndqC2hjt6Xa3cUDsjEpm7D6u9kJRFEPmdL avg TPS:0.026082
✅ 2Qb172jGBtjTTLhrzYD8ZLatjg6FFmbiFSP6CBq2Xy4aBV2WxL actor: token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp units: 488 summary (*actions.CreateOrder): [1.000000000 TKN -> 10 27grFs9vE2YP9kwLM5hQJGLDvqEY9ii71zzdoRHNGC4Appavug (supply: 50 27grFs9vE2YP9kwLM5hQJGLDvqEY9ii71zzdoRHNGC4Appavug)]
height:14 txs:1 units:1536 root:2vqraWhyd98zVk2ALMmbHPApXjjvHpxh4K4u1QhSb6i3w4VZxM avg TPS:0.030317
✅ 2H7wiE5MyM4JfRgoXPVP1GkrrhoSXL25iDPJ1wEiWRXkEL1CWz actor: token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp units: 1536 summary (*actions.FillOrder): [2.000000000 TKN -> 20 27grFs9vE2YP9kwLM5hQJGLDvqEY9ii71zzdoRHNGC4Appavug (remaining: 30 27grFs9vE2YP9kwLM5hQJGLDvqEY9ii71zzdoRHNGC4Appavug)]
height:15 txs:1 units:464 root:u2FyTtup4gwPfEFybMNTgL2svvSnajfGH4QKqiJ9vpZBSvx7q avg TPS:0.036967
✅ Lsad3MZ8i5V5hrGcRxXsghV5G1o1a9XStHY3bYmg7ha7W511e actor: token1rvzhmceq997zntgvravfagsks6w0ryud3rylh4cdvayry0dl97nsjzf3yp units: 464 summary (*actions.CloseOrder): [orderID: 2Qb172jGBtjTTLhrzYD8ZLatjg6FFmbiFSP6CBq2Xy4aBV2WxL]
```

### Running a Load Test
_Before running this demo, make sure to stop the network you started using
`killall avalanche-network-runner`._

The `tokenvm` load test will provision 5 `tokenvms` and process 500k transfers
on each between 10k different accounts.

```bash
./scripts/tests.load.sh
```

_This test SOLELY tests the speed of the `tokenvm`. It does not include any
network delay or consensus overhead. It just tests the underlying performance
of the `hypersdk` and the storage engine used (in this case MerkleDB on top of
Pebble)._

#### Measuring Disk Speed
This test is extremely sensitive to disk performance. When reporting any TPS
results, please include the output of:

```bash
./scripts/tests.disk.sh
```

_Run this test RARELY. It writes/reads many GBs from your disk and can fry an
SSD if you run it too often. We run this in CI to standardize the result of all
load tests._

## Zipkin Tracing
To trace the performance of `tokenvm` during load testing, we use `OpenTelemetry + Zipkin`.

To get started, startup the `Zipkin` backend and `ElasticSearch` database (inside `hypersdk/trace`):
```bash
docker-compose -f trace/zipkin.yml up
```
Once `Zipkin` is running, you can visit it at `http://localhost:9411`.

Next, startup the load tester (it will automatically send traces to `Zipkin`):
```bash
TRACE=true ./scripts/tests.load.sh
```

When you are done, you can tear everything down by running the following
command:
```bash
docker-compose -f trace/zipkin.yml down
```

## Creating a Devnet
_In the world of Avalanche, we refer to short-lived, test-focused Subnets as devnets._

Using [avalanche-ops](https://github.com/ava-labs/avalanche-ops), we can create a private devnet (running on a
custom Primary Network with traffic scoped to the deployer IP) across any number of regions and nodes
in ~30 minutes with a single script.

### Step 1: Install Dependencies
#### Install and Configure `aws-cli`
Install the [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html). This is used to
authenticate to AWS and manipulate CloudFormation.

Once you've installed the AWS CLI, run `aws configure sso` to login to AWS locally. See
[the avalanche-ops documentation](https://github.com/ava-labs/avalanche-ops#permissions) for additional details.
Set a `profile_name` when logging in, as it will be referenced directly by avalanche-ops commands. **Do not set
an SSO Session Name (not supported).**

#### Install `yq`
Install `yq` using [Homebrew](https://brew.sh/). `yq` is used to manipulate the YAML files produced by
`avalanche-ops`.

You can install `yq` using the following command:
```bash
brew install yq
```

### Step 2: Deploy Devnet on AWS
Once all dependencies are installed, we can create our devnet using a single script. This script deploys
10 validators (equally split between us-west-2, us-east-2, and eu-west-1):
```bash
./scripts/deploy.devnet.sh
```

_When devnet creation is complete, this script will emit commands that can be used to interact
with the devnet (i.e. tx load test) and to tear it down._

#### Configuration Options
* `--arch-type`: `avalanche-ops` supports deployment to both `amd64` and `arm64` architectures
* `--anchor-nodes`/`--non-anchor-nodes`: `anchor-nodes` + `non-anchor-nodes` is the number of validators that will be on the Subnet, split equally between `--regions` (`anchor-nodes` serve as bootstrappers on the custom Primary Network, whereas `non-anchor-nodes` are just validators)
* `--regions`: `avalanche-ops` will automatically deploy validators across these regions
* `--instance-types`: `avalanche-ops` allows instance types to be configured by region (make sure it is compatible with `arch-type`)
* `--upload-artifacts-avalanchego-local-bin`: `avalanche-ops` allows a custom AvalancheGo binary to be provided for validators to run

## Future Work
_If you want to take the lead on any of these items, please
[start a discussion](https://github.com/AnomalyFi/hypersdk/discussions) or reach
out on the Avalanche Discord._

* Document GUI/Faucet/Feed
* Add more config options for determining which order books to store in-memory
* Add option to CLI to fill up to some amount of an asset as long as it is
  under some exchange rate (trading agent command to provide better UX)
* Add expiring order support (can't fill an order after some point in time but
  still need to explicitly close it to get your funds back -> async cleanup is
  not a good idea)

<br>
<br>
<br>
<p align="center">
  <a href="https://github.com/AnomalyFi/hypersdk"><img width="40%" alt="powered-by-hypersdk" src="assets/hypersdk.png"></a>
</p>
