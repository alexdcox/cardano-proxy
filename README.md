# Cardano Proxy

Writes raw binary request/responses between cardano node and a respective client
to hex files.

Suggested client:  
https://github.com/cardano-foundation/cf-ledger-sync

## Building

```
go build
```

## Usage

```
cardano-proxy --node 127.0.0.1:7731 --proxy 127.0.0.1:7732
```