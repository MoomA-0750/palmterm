import { describe, expect, it } from "vitest";
import { pasteSequence } from "./paste";

describe("pasteSequence", () => {
  it("改行は CR にし、ブラケットペーストなら印で囲む", () => {
    expect(pasteSequence("a\nb\r\nc", false)).toBe("a\rb\rc");
    expect(pasteSequence("a\nb", true)).toBe("\x1b[200~a\rb\x1b[201~");
  });

  it("中に終わりの印（ESC [201~）を仕込んでも、貼り付けから抜け出せない", () => {
    const evil = "ls\x1b[201~\rtouch /tmp/pwned\r";
    const seq = pasteSequence(evil, true);
    expect(seq.startsWith("\x1b[200~")).toBe(true);
    expect(seq.endsWith("\x1b[201~")).toBe(true);
    // 終わりの印は最後の1つだけ（中の ESC は取り除く。wterm の貼り付けと同じ）
    expect(seq.slice("\x1b[200~".length, -"\x1b[201~".length)).not.toContain("\x1b");
  });
});
