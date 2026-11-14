import { Transform } from 'node:stream'

export class PCMTransform extends Transform {
  constructor() {
    super()
    this.pending = Buffer.alloc(0)
  }

  _transform(chunk, _, callback) {
    const data = Buffer.concat([this.pending, chunk])
    const bytes = data.length - data.length % 8
    const output = Buffer.alloc(bytes / 2)
    for (let offset = 0; offset < bytes; offset += 4) {
      const sample = data.readFloatLE(offset)
      if (!Number.isFinite(sample) || Math.abs(sample) > 1) {
        return callback(new Error('Invalid stereo PCM sample'))
      }
      output.writeInt16LE(Math.min(32767, Math.round(sample * 32768)), offset / 2)
    }
    this.pending = Buffer.from(data.subarray(bytes))
    callback(null, output)
  }

  _flush(callback) {
    callback(this.pending.length ? new Error('Incomplete stereo PCM frame') : null)
  }
}
