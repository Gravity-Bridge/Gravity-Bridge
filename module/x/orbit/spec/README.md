<!--
order: 0
title: Orbit Overview
parent:
  title: "orbit"
-->

# `Orbit`

## Abstract
An Integration Test module, providing easy manipulation and monitoring of a running test chain.

Simply modify Orbit to contain your chain's keepers and wire up Msgs to modify the Keeper state.
Optionally expose hidden module information via the Query service.
During an integration test, submit a Msg to trigger your Integration Test case.

Additionally, Orbit makes it easy to test failing invariants through its very poorly designed failing-invariant.

It's the perfect place to put horrible code!