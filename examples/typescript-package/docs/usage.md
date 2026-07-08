# Usage

`example-typescript-package` is a tiny arithmetic library used to demonstrate
CodeSteward. It exposes a tokenizer, an expression parser, and a small cache.

## Parsing an expression

```ts
import { parse } from "example-typescript-package";

parse("1 + 2 * 3"); // => 7
parse("(1 + 2) * 3"); // => 9
```

## Tokenizing

```ts
import { tokenize } from "example-typescript-package";

tokenize("1 + 2");
// => [
//   { kind: "number", value: "1" },
//   { kind: "plus", value: "+" },
//   { kind: "number", value: "2" },
// ]
```

## Caching results

```ts
import { LruCache } from "example-typescript-package";

const cache = new LruCache<string, number>(2);
cache.set("a", 1);
cache.set("b", 2);
cache.get("a"); // marks "a" as most recently used
cache.set("c", 3); // evicts "b"
cache.has("b"); // => false
```
