muse
====

[![GoDoc](https://godoc.org/lukechampine.com/muse?status.svg)](https://godoc.org/lukechampine.com/muse)
[![Go Report Card](https://goreportcard.com/badge/lukechampine.com/muse)](https://goreportcard.com/report/lukechampine.com/muse)

`muse` is a contract server for Sia. It provides contracts to apps like
[user](https://github.com/lukechampine/user) so that they can store and retrieve
files on Sia hosts. This means that you can administrate your contracts (choose
hosts, form contracts with them, and renew those contracts periodically) in a
single place and use them on any of your devices. Alternatively, you can offload
this responsibility to a third party who runs a `muse` server on your behalf.

To run a `muse` server, you will need access to a [`shard`](https://github.com/lukechampine/shard)
server (to lookup host IP addresses) and a [`walrus`](https://github.com/lukechampine/walrus)
server (to fund contract transactions). For convenience, I run public instances
of these services. My shard server address is http://shard.lukechampine.com, and
you can get a personal `walrus` server by visiting https://narwal.lukechampine.com.

API documentation can be found [here](https://lukechampine.com/docs/muse).
