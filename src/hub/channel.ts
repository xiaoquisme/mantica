/**
 * Go channel 风格的异步可迭代队列。
 * 支持多 writer、单 reader，close 后结束迭代。
 */
export class Channel<T> implements AsyncIterable<T> {
  private buffer: T[] = [];
  private closed = false;

  private readers: Array<{
    resolve: (result: IteratorResult<T>) => void;
  }> = [];

  get isClosed(): boolean {
    return this.closed;
  }

  get size(): number {
    return this.buffer.length;
  }

  /** 发送值到 channel。channel 已关闭时返回 false。 */
  send(value: T): boolean {
    if (this.closed) return false;

    const reader = this.readers.shift();
    if (reader) {
      reader.resolve({ value, done: false });
      return true;
    }

    this.buffer.push(value);
    return true;
  }

  /** 关闭 channel，唤醒所有等待中的 reader。 */
  close(): void {
    if (this.closed) return;
    this.closed = true;

    for (const reader of this.readers) {
      reader.resolve({ value: undefined as T, done: true });
    }
    this.readers = [];
  }

  [Symbol.asyncIterator](): AsyncIterator<T> {
    return {
      next: (): Promise<IteratorResult<T>> => {
        if (this.buffer.length > 0) {
          const value = this.buffer.shift()!;
          return Promise.resolve({ value, done: false });
        }

        if (this.closed) {
          return Promise.resolve({ value: undefined as T, done: true });
        }

        return new Promise<IteratorResult<T>>((resolve) => {
          this.readers.push({ resolve });
        });
      },
    };
  }
}
