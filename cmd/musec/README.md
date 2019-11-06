# musec

`musec` is a client for `muse`. It lets you form, renew, and organize contracts
on a `muse` server.


## Scanning for Hosts

Before you can form contracts, you need to choose your hosts. You can get a
ranked list of hosts by running `siac hostdb -v`. The longer `siad` has been
running, the more accurate these rankings will be. You can also consult a
service like [SiaStats](https://siastats.info/hosts), which regularly benchmarks
hosts and measures their true performance.

You'll need the public key of each host you want to use. Host public keys look
like this:

```
ed25519:706715a4f37fda29f8e06b867c5df3f139f6ed93c18d99a5665eb66a5fab6009
```

Since these keys are long and unwieldy, `musec` lets you use an abbreviated
form. In the abbreviated form, the `ed25519:` prefix is dropped, and only the
first few characters of the key are retained. The key above, for example, could
be shortened to `706715a4`. Like git hashes, you only need enough characters to
ensure that the key is unambiguous; eight is a safe choice.

Use the `scan` command to query a host's settings:

```
$ musec scan [hostkey] [filesize] [duration]
```

`filesize` is the total amount of data stored, and `duration` is the number of
blocks the data is stored for. The command will print the amount of funds necessary
to form the desired contract, as well as a breakdown of individual costs:

```
$ musec scan 5de2fed25 1TB 4000
Scanned host in 469.358437ms

Public Key:      ed25519:5de2fed25c48322484028dcb3f76717fff31e6716bdcc5d300581da425274123
IP Address:      example.com:9982
Latency:         172.960484ms

Cost Summary:
Storage Cost:     545.3 SC ██████████████████████▏
Upload Cost:       5.89 SC ▏
Download Cost:    883.4 SC ███████████████████████████████████▉
Host Fee:         2.559 SC ▏
Siafund Fee:      98.59 SC ████

Contract Inputs:
Renter Funds:     1.437 KS ███████████████████████████████████▌
Host Collateral:  1.091 KS ██████████████████████████▉
Collateral Ratio:    0.76x

Contract Value:   1.437 KS (pass this value to the 'form' and 'renew' commands)
Transaction Cost: 1.536 KS (this is the amount that will be deducted from your wallet)
```

Note that the "Download Cost" is the cost to download the data *once*.


## Forming Contracts

If the host settings are reasonable, you can proceed to form a contract:

```
$ musec form [hostkey] [funds] [endheight]
```

`hostkey` is the public key of the host; `funds` is the amount of siacoins the
contract will store; and `endheight` is the height at which the host is no
longer obligated to store the data.

Note that, in the above command, `funds` does not include the transaction fee,
the host's contract fee, or the siafund tax. `funds` is simply the number of
coins in the renter's half of the payment channel, i.e. the amount reserved for
paying the host when uploading and downloading. Use the "Contract Funds" field
from the `scan` command to estimate how much to spend.

To view your contracts, run `musec contracts`.

## Renewing Contracts

Once you have a contract, renewing is easy:

```
$ musec renew [contract] [funds] [endheight]
```

`contract` is the ID of the original contract. Unlike host keys, this ID must
not be abbreviated.

The host may be offline when you attempt to renew, in which case you will have
to try again later. If the contract is not renewed before it expires, the host
will delete any data associated with the contract. For this reason, it is
recommended that you first attempt to renew a contract at least 1000 blocks
(approx. 1 week) before it expires.


## Host Sets

When you renew a contract, what happens to the previous contract? In a sense,
the answer is "nothing." A `muse` server stores all of its contracts together in
one big bag, even after they've expired. But when you *use* your contracts, you
generally want to use the most-recently-renewed version of a contract. You might
also want to maintain multiple groups of contracts, possibly overlapping, so
that you can use different hosts for different tasks. For example, you might
have a "cheap hosts" group and a "fast hosts" group. `muse` addresses this need
with "host sets."

A host set is simply a list of public keys, grouped under a user-specified name.
You can create a host set like so:

```
$ musec hosts create [name] [host set]
```

Where `host set` is a set of comma-separated host public keys.

Now, you can run `musec contracts` with an additional argument: the name of your
host set. This will display only the contracts that have been formed with hosts
in the set. Furthermore, only the most recent contract for each host will be
included (where "most recent" means "highest end height"). Applications that get
their contracts from a `muse` server will generally request contracts in this
way, rather than requesting the full list of all historical contracts.

To modify a host set, just run the `create` command again with the new set. You
can also delete a host set with the `delete` command.
