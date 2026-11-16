import { describe, expect, it } from "vitest";
import { normalizeFolderName, validateFolderName } from "./validation";

describe("folder name validation", () => {
  it("mirrors Unicode edge trimming and code point limits", () => {
    expect(normalizeFolderName("\u00a0 Topic \u2003")).toBe("Topic");
    expect(validateFolderName(" \n ")).toBe("Поле обязательно");
    expect(validateFolderName("🙂".repeat(80))).toBe("");
    expect(validateFolderName("🙂".repeat(81))).toBe("Не больше 80 символов");
  });
});
