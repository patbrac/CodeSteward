import { Token, tokenize } from "./tokenize";

// Recursive-descent parser that evaluates a tiny arithmetic expression.
// grammar:
//   expr   := term (("+" | "-") term)*
//   term   := factor (("*" | "/") factor)*
//   factor := number | "(" expr ")"

export function parse(input: string): number {
  const tokens = tokenize(input);
  let pos = 0;

  const peek = (): Token | undefined => tokens[pos];
  const advance = (): Token | undefined => tokens[pos++];

  function factor(): number {
    const tok = advance();
    if (tok === undefined) {
      throw new Error("unexpected end of input");
    }
    if (tok.kind === "number") {
      return Number(tok.value);
    }
    if (tok.kind === "lparen") {
      const value = expr();
      const close = advance();
      if (close === undefined || close.kind !== "rparen") {
        throw new Error("expected closing parenthesis");
      }
      return value;
    }
    throw new Error(`unexpected token: ${tok.value}`);
  }

  function term(): number {
    let value = factor();
    for (let t = peek(); t !== undefined && (t.kind === "star" || t.kind === "slash"); t = peek()) {
      advance();
      const rhs = factor();
      value = t.kind === "star" ? value * rhs : value / rhs;
    }
    return value;
  }

  function expr(): number {
    let value = term();
    for (let t = peek(); t !== undefined && (t.kind === "plus" || t.kind === "minus"); t = peek()) {
      advance();
      const rhs = term();
      value = t.kind === "plus" ? value + rhs : value - rhs;
    }
    return value;
  }

  const result = expr();
  if (pos !== tokens.length) {
    throw new Error("unexpected trailing tokens");
  }
  return result;
}
