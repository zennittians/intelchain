# Blacklist info

The black list is a newline delimited file of wallet addresses. It can also support comments with the `#` character.

## Default Location

By default, the intelchain binary looks for the file `./.itc/blaklist.txt`.

## Details

Each transaction added to the tx-pool has its `to` and `from` address checked against this blacklist. 
If there is a hit, the transaction is considered invalid and is dropped from the tx-pool.