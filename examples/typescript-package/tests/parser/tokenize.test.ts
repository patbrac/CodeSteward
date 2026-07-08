import { tokenize } from "../../src/parser/tokenize";

describe("tokenize", () => {
  it("tokenizes a bare number", () => {
    expect(tokenize("42")).toEqual([{ kind: "number", value: "42" }]);
  });

  it("tokenizes an addition expression", () => {
    expect(tokenize("1 + 2")).toEqual([
      { kind: "number", value: "1" },
      { kind: "plus", value: "+" },
      { kind: "number", value: "2" },
    ]);
  });

  it("tokenizes parentheses and operators", () => {
    expect(tokenize("(3*4)")).toEqual([
      { kind: "lparen", value: "(" },
      { kind: "number", value: "3" },
      { kind: "star", value: "*" },
      { kind: "number", value: "4" },
      { kind: "rparen", value: ")" },
    ]);
  });

  it("ignores surrounding whitespace", () => {
    expect(tokenize("  7  ")).toEqual([{ kind: "number", value: "7" }]);
  });

  it("throws on an unexpected character", () => {
    expect(() => tokenize("1 % 2")).toThrow();
  });
});
