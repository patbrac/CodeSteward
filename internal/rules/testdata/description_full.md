## Summary

This change refactors the tokenizer to be allocation-free on the hot path,
which meaningfully reduces GC pressure for large inputs.

## Motivation

Profiling showed the tokenizer accounted for a large share of parse time.

**Test plan**

- Added unit tests for the new tokenizer states.
- Ran the existing parser suite locally.

Closes #128.
