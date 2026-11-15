# libdave Go binding

This directory contains [`github.com/disgoorg/godave/libdave` v0.3.0](https://github.com/disgoorg/godave/tree/libdave/v0.3.0/libdave),
under its [Apache 2.0 license](LICENSE). It is only the Go/C binding: Discord's
libdave and its MLS implementation remain separate upstream native binaries.
The root `go.mod` replaces the binding module with this copy.

The binding passes callback userdata to libdave, which retains it beyond the
CGO call. The upstream binding passes pointers to Go memory without pinning;
this copy instead stores `cgo.Handle` values in C-allocated memory until the
callback is no longer usable. The fingerprint callback channel is buffered so
a synchronous callback cannot block its own caller.
The copy also keeps finalizer-owned native handles alive throughout C calls,
including the asynchronous fingerprint request, and fixes a decryptor
finalizer that captured its own object and prevented cleanup.

Remove this replacement when an upstream binding safely owns retained callback
userdata and native handle lifetimes. When updating it, review all callback
lifetimes, keep the license, and run the binding tests, Go race tests and the
live Discord browser journey.
The native library is pinned separately by `scripts/install-libdave.sh`.
