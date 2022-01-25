## 0002 -- State Signing Algorithm

## Status

Accepted

## Context

We require an algorithm for digitally signing a state. A state is typed data, so the algorithm must encode it, possibly hash it, and then sign using e.g. ECDSA. The algorithm should protect user funds in the light of possibly malicious on-chain code.

## Decision

Our state signing scheme involves encoding a data structure into a string of bytes, hashing those using [`keccak256`](https://en.wikipedia.org/w/index.php?title=Keccak-256&redirect=no) and signing the hash using an ephemeral [ECDSA](https://en.wikipedia.org/wiki/Elliptic_Curve_Digital_Signature_Algorithm) key.

**This key should not be used in any other protocol.**

Nitro clients should verify the decoding from the contract where funds are locked.

## Consequences

Signing a hash is important because it protects against leakage of the private key -- since the hash is unpredictable it cannot be crafted maliciously by an adversary (even if the preimage has been prepared by that adversary).

There remains a potential issue with signatures being reused in unintended ways.

Consider an encoding collision `p = encode(a,b) = encode'(a',b') = p'`. Put otherwise, the bytes `p` could be encoded / decoded in two different ways (perhaps the intended way plus another unintended way), giving rise to distinct variables `a' != a, b' != b`. If these distinct sets of variables and encoding functions result in the same preimage for the hashing and signing steps, the same signature could be "valid" for both sets.

According to this ADR, Nitro clients should verify the decoding from the contract where funds are locked. Notwithstanding things like upgradeable contracts, if we use `abi.encode` this decoding is deterministic and therefore totally predictable. (Contrast this with using something like `abi.encodePacked`, where multiple inputs encode to the same bytes -- as a consequence there is no decoder for `abi.encodePacked` as the there is no unique pre-image).

A signature on `h(p)` might get "replayed" to a contract with a different decoder, with unintended effects -- but as long as no funds are ever sent there, there should be no issue. We do need to ensure that the signature is generated by a truly ephemeral key, though -- the only place where that key should have meaning is for signing nitro state channel updates.

Hashing the outcome and appdata parts of the state separately, and mixing them with the remaining fields before hashing and signing, would make it much harder (effectively impossible) to engineer the malicious decoder contract. But this extra protection is unnecessary given the algorithm above.

For each signature made with a given private key, the owner of said key should be checking on the consequences of that signature for each and every contract (on each and every chain) which holds funds or other important state for the owner and can be controlled to some extent by that key. This includes the native assets and accounts of those chains. They do these checks on the data to be signed, and decline to sign if they spot an issue.

Such contracts/chains might only be deployed after some signatures have been made with that key. So the owner of the key needs to also check all previous signatures that have been made with that key against the new chain/contract before depositing funds there. This is pretty infeasible, too, so the owner should select a new private key for every new protocol that they enter\* in order to remain safe.

An example attack would be to take the data which was signed during execution of an existing protocol (for example, state channels) and convince the signer to partake in a new protocol backed by a contract which has the "specially crafted encoder" that you mention. If the signer deposits money it could be at risk by having the existing signatures (which are already in the wild) played onto the new contract.