// 端末に出力があったときに、少し待ってから何かを取り直す（tmux のタブ）。
// 出力が続いている間も一定の間隔より頻繁には取り直さず、結果が変わらない間は間隔を広げていく
// （出力が多いと毎回 tmux のコマンドがサーバーで動くため）。取り直している最中に頼まれたら、終わったあとに1回だけ取り直す。

const DELAY_MS = 250; // 出力があってから取り直すまで
const MIN_INTERVAL_MS = 1000;
const MAX_INTERVAL_MS = 8000;

export class Refresher {
  private timer: ReturnType<typeof setTimeout> | undefined;
  private last = -Infinity;
  private interval = MIN_INTERVAL_MS;
  private running: Promise<void> | null = null;
  private again = false;
  private generation = 0; // reset のたびに進める。前に始めた取り直しの結果では間隔を変えない

  /** fetch は取り直しを行い、前と結果が変わったら true を返す。 */
  constructor(private fetch: () => Promise<boolean>) {}

  /** 端末に出力があったときに呼ぶ。 */
  outputSeen() {
    if (this.timer !== undefined) return;
    const wait = Math.max(DELAY_MS, this.last + this.interval - Date.now());
    this.timer = setTimeout(() => {
      this.timer = undefined;
      void this.refresh();
    }, wait);
  }

  /** 間隔を最短に戻す（操作したあとなど、変化がありそうなとき）。 */
  reset() {
    this.generation++;
    this.interval = MIN_INTERVAL_MS;
    // 広い間隔で予約してあった取り直しは、次の出力で組み直す。
    clearTimeout(this.timer);
    this.timer = undefined;
  }

  /** すぐに取り直す。取り直している最中なら、終わったあとにもう一度。 */
  refresh(): Promise<void> {
    if (this.running) {
      this.again = true;
      return this.running;
    }
    this.last = Date.now();
    const generation = this.generation;
    this.running = this.fetch()
      .then(
        (changed) => {
          if (generation !== this.generation) return;
          this.interval = changed ? MIN_INTERVAL_MS : Math.min(this.interval * 2, MAX_INTERVAL_MS);
        },
        () => {},
      )
      .finally(() => {
        this.running = null;
        if (this.again) {
          this.again = false;
          void this.refresh();
        }
      });
    return this.running;
  }
}
