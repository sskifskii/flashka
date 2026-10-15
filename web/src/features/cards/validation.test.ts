import { describe, expect, it } from "vitest";
import { normalizeSide, validateSide } from "./validation";

describe("card side validation", () => {
  it("trims Unicode whitespace and preserves internal whitespace", () => {
    expect(normalizeSide("\u00a0\u2003front \n back\u2003")).toBe(
      "front \n back",
    );
  });
  it("counts Unicode code points", () => {
    expect(validateSide("😀".repeat(2000))).toBe("");
    expect(validateSide("😀".repeat(2001))).toMatch(/2000/);
    expect(validateSide(" \u00a0 ")).toMatch(/обязательно/);
  });
});
