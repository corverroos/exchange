# Exchange

Exchange is a "foreign exchange market" PoC using golang, [reflex](https://github.com/luno/reflex) and mysql as backend. 
It provides a single market/pair and supports: market orders, limit orders (including post only) and cancelling orders.

The API is very simple and queries the DB synchronously. 
Matching of orders and creating of trades is done asynchronously. 

The aim to showcase the performance and reliability of reflex event streams to power mission critical applications.
It also shows the benefits of an "online deterministic matching engine".

## Overview

Exchange has three main db tables.

- `orders`: Represents the order state machine. States are `pending, posted, cancelling, completed`.
- `results`: Append only log of matching results.
- `trades`: Trades populated from match results.

The following reflex tables are also present:
 - `order_events`: Events of orders state changes. These drive the matching engine.
 - `result_events`: Due to reflex not supporting append only tables directly, a event table is used. It matches the result table one-to-one.
 - `cursors`: Reflex consumer cursor store.

## API

An exchange needs liquidity, this is provided by orders, these are created via the API.

The orders API has methods `create order`, `cancel order` which update the orders table state machine. The state machine creates events for each state change.

## Matching engine

The matching engine consists of three concurrent processes linked by golang channels:
 - Input: Reflex streams order events which are transformed into matcher commands and piped into the input channel. 
 - Matching: The matcher reads commands from the input channel, applies it to the order book and pipes the result including any trades into the output channel. 
 - Output: Results are read from the output channel and stored in the results append only log table.
 
Another reflex consumer streams results and updates the order state machine and inserts any trades. 
 
## Performance

> Note: Tests require local mysql with root user without password. It uses default mysql socket.

The following is the results of [TestPerformance](./exchange_test.go) as run on a MacBook Pro (2,3 GHz, 8 GB RAM)
with local MySQL 5.7 on SSD.

The test creates 10000 pseudorandom (deterministic) orders: 10% post only, 70% limit, 20% market orders
of which 20% of the limit orders are cancelled and 50% buy vs sell. It starts the exchange (and starts timing) when 
the post only orders are inserted. It also inserts one last market order after all other orders have been inserted (and cancelled).
It stops timing when the last market order has been processed. The resulting rate the is the number of commands processed
divided by the duration. Processed means that the matching result has been stored in the append only result table.

The graph shows the history of performance improvements (from old to new).

| Commit        | Rate (cmds/s) | Comment  
| ------------- |--------------:| -----|
| d2067a8       | 500  | Initial implementation. Reads each order per event and stores each result sequentially.
| d897059       | 1500 | Stores batches of results per row in results table.

The following things could improve performance:
 - It seems like reading the orders is the bottleneck since the channels are most empty. Avoid reading each order by inserting metadata in events.
 - Adding support to reflex for streaming directly from append-only table removes need to create result events. 
 - For large order books, improve the matching performance using heaps instead of slices.
