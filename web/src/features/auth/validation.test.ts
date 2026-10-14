import { describe, expect, it } from "vitest";
import { credentialsSchema } from "./validation";
describe("credentials validation", () => {
  it.each([
    ["Ab3", "12345678", true],
    ["ab", "12345678", false],
    ["_abc", "12345678", false],
    ["abc", "1234567", false],
    ["abc", "1234567😀", true],
    ["abc", "123456e\u0301", true],
  ])("validates %s", (login, password, valid) => {
    expect(credentialsSchema.safeParse({ login, password }).success).toBe(
      valid,
    );
  });
});
