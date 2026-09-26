// サーバーとの WebSocket。キー入力はバイナリ、サイズ変更は JSON のテキストで送る。
// 切れたら間隔を広げながらつなぎ直す（tmux のセッションはサーバー側に残る）。

export type ConnectionStatus = "connecting" | "open" | "closed";

const encoder = new TextEncoder();

export class Connection {
  private ws: WebSocket | null = null;
  private retryDelay = 500;
  private retryTimer: number | undefined;

  onOutput: (data: Uint8Array) => void = () => {};
  onOpen: () => void = () => {};
  onStatus: (status: ConnectionStatus) => void = () => {};

  constructor(private size: () => { cols: number; rows: number }) {
    // 裏から戻ったとき、切れていればすぐつなぎ直す（スマホは裏で通信が切られる）。
    document.addEventListener("visibilitychange", () => {
      if (document.visibilityState === "visible" && !this.isOpen()) this.connectNow();
    });
  }

  connect() {
    this.connectNow();
  }

  private connectNow() {
    window.clearTimeout(this.retryTimer);
    if (this.ws && this.ws.readyState === WebSocket.CONNECTING) return;
    this.ws?.close();
    const { cols, rows } = this.size();
    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    const ws = new WebSocket(`${proto}//${location.host}/ws?cols=${cols}&rows=${rows}`);
    ws.binaryType = "arraybuffer";
    this.ws = ws;
    this.onStatus("connecting");

    ws.onopen = () => {
      this.retryDelay = 500;
      this.onStatus("open");
      this.onOpen();
      // つないでいる間にサイズが変わっていたら合わせる。
      const now = this.size();
      if (now.cols !== cols || now.rows !== rows) this.resize(now.cols, now.rows);
    };
    ws.onmessage = (e) => {
      if (e.data instanceof ArrayBuffer) this.onOutput(new Uint8Array(e.data));
    };
    ws.onclose = () => {
      if (this.ws !== ws) return;
      this.ws = null;
      this.onStatus("closed");
      this.retryTimer = window.setTimeout(() => this.connectNow(), this.retryDelay);
      this.retryDelay = Math.min(this.retryDelay * 2, 10_000);
    };
  }

  isOpen(): boolean {
    return this.ws?.readyState === WebSocket.OPEN;
  }

  send(data: string | Uint8Array) {
    if (!this.isOpen() || data.length === 0) return;
    this.ws!.send(typeof data === "string" ? encoder.encode(data) : (data as Uint8Array<ArrayBuffer>));
  }

  resize(cols: number, rows: number) {
    if (!this.isOpen()) return;
    this.ws!.send(JSON.stringify({ type: "resize", cols, rows }));
  }

  /** tmux のコピーモードでペインをスクロールする。lines が負なら古い方（上）へ。 */
  scroll(lines: number) {
    if (!this.isOpen() || lines === 0) return;
    this.ws!.send(JSON.stringify({ type: "scroll", lines }));
  }
}
