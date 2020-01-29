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
 
 Another reflex consumer streams results and update order state machine and inserts any trades. 
 
 ## Performance
 
 The current implementation processes around 500 commands per second on a MacBook pro. See exchange_test.go#TestPerformance.
