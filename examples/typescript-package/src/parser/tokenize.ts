// Simple expression tokenizer for a tiny calculator language.
// Supports non-negative integers, the operators + - * / and parentheses.

export type TokenKind =
  | "number"
  | "plus"
  | "minus"
  | "star"
  | "slash"
  | "lparen"
  | "rparen";

export interface Token {
  kind: TokenKind;
  value: string;
}

const SINGLE: Record<string, TokenKind> = {
  "+": "plus",
  "-": "minus",
  "*": "star",
  "/": "slash",
  "(": "lparen",
  ")": "rparen",
};

function isDigit(ch: string): boolean {
  return ch >= "0" && ch <= "9";
}

export function tokenize(input: string): Token[] {
  const tokens: Token[] = [];
  let i = 0;
  while (i < input.length) {
    const ch = input[i];
    if (ch === " " || ch === "\t" || ch === "\n") {
      i += 1;
      continue;
    }
    if (isDigit(ch)) {
      let num = "";
      while (i < input.length && isDigit(input[i])) {
        num += input[i];
        i += 1;
      }
      tokens.push({ kind: "number", value: num });
      continue;
    }
    const kind = SINGLE[ch];
    if (kind === undefined) {
      throw new Error(`unexpected character: ${ch}`);
    }
    tokens.push({ kind, value: ch });
    i += 1;
  }
  return tokens;
}
