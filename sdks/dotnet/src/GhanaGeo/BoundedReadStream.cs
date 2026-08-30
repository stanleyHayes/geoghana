namespace GhanaGeo;

internal sealed class ResponseLimitExceededException : IOException
{
    public ResponseLimitExceededException(long maximumBytes) : base($"Response exceeded the configured {maximumBytes}-byte limit.") => MaximumBytes = maximumBytes;
    public long MaximumBytes { get; }
}

internal sealed class BoundedReadStream(Stream inner, long maximumBytes) : Stream
{
    private long consumed;
    public override bool CanRead => inner.CanRead;
    public override bool CanSeek => false;
    public override bool CanWrite => false;
    public override long Length => throw new NotSupportedException();
    public override long Position { get => consumed; set => throw new NotSupportedException(); }
    public override void Flush() => throw new NotSupportedException();
    public override long Seek(long offset, SeekOrigin origin) => throw new NotSupportedException();
    public override void SetLength(long value) => throw new NotSupportedException();
    public override void Write(byte[] buffer, int offset, int count) => throw new NotSupportedException();

    public override int Read(byte[] buffer, int offset, int count)
    {
        var read = inner.Read(buffer, offset, Allowed(count));
        Count(read);
        return read;
    }

    public override async ValueTask<int> ReadAsync(Memory<byte> buffer, CancellationToken cancellationToken = default)
    {
        var read = await inner.ReadAsync(buffer[..Allowed(buffer.Length)], cancellationToken).ConfigureAwait(false);
        Count(read);
        return read;
    }

    protected override void Dispose(bool disposing)
    {
        if (disposing) inner.Dispose();
        base.Dispose(disposing);
    }

    public override async ValueTask DisposeAsync()
    {
        await inner.DisposeAsync().ConfigureAwait(false);
        await base.DisposeAsync().ConfigureAwait(false);
    }

    private int Allowed(int requested)
    {
        var remainingWithProbe = maximumBytes - consumed + 1;
        return (int)Math.Min(requested, Math.Max(1, remainingWithProbe));
    }

    private void Count(int read)
    {
        consumed += read;
        if (consumed > maximumBytes) throw new ResponseLimitExceededException(maximumBytes);
    }
}
